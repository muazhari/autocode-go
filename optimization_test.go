package autocode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Mock Application satisfying the OptimizationApplication interface
type MockApplication struct {
	evaluateFunc func(ctx *Optimization) *OptimizationEvaluateRunResponse
}

func (m *MockApplication) Evaluate(ctx *Optimization) *OptimizationEvaluateRunResponse {
	if m.evaluateFunc != nil {
		return m.evaluateFunc(ctx)
	}
	return &OptimizationEvaluateRunResponse{
		Objectives:            []float64{1.0},
		InequalityConstraints: []float64{},
		EqualityConstraints:   []float64{},
	}
}

func dummyFunction(ctx *Optimization, args ...any) any {
	return "result"
}

func TestVariablesCreationAndMap(t *testing.T) {
	bin := NewOptimizationBinary("v_bin")
	if bin.Id != "v_bin" || bin.Type != VARIABLE_BINARY {
		t.Errorf("NewOptimizationBinary initialization failed")
	}
	binMap := bin.Map()
	if binMap["id"] != "v_bin" || binMap["type"] != VARIABLE_BINARY {
		t.Errorf("OptimizationBinary Map() serialization failed")
	}

	integer := NewOptimizationInteger("v_int", 1, 10)
	if integer.Id != "v_int" || integer.Type != VARIABLE_INTEGER || integer.Bounds[0] != 1 || integer.Bounds[1] != 10 {
		t.Errorf("NewOptimizationInteger initialization failed")
	}
	integerMap := integer.Map()
	if integerMap["id"] != "v_int" || integerMap["type"] != VARIABLE_INTEGER {
		t.Errorf("OptimizationInteger Map() serialization failed")
	}

	realVar := NewOptimizationReal("v_real", -1.5, 1.5)
	if realVar.Id != "v_real" || realVar.Type != VARIABLE_REAL || realVar.Bounds[0] != -1.5 || realVar.Bounds[1] != 1.5 {
		t.Errorf("NewOptimizationReal initialization failed")
	}
	realMap := realVar.Map()
	if realMap["id"] != "v_real" || realMap["type"] != VARIABLE_REAL {
		t.Errorf("OptimizationReal Map() serialization failed")
	}

	choice := NewOptimizationChoice("v_choice", []any{dummyFunction, int64(10), float64(2.5), true})
	if choice.Id != "v_choice" || choice.Type != VARIABLE_CHOICE {
		t.Errorf("NewOptimizationChoice initialization failed")
	}
	if len(choice.Options) != 4 {
		t.Errorf("Expected 4 options in choice variable, got %d", len(choice.Options))
	}
	choiceMap := choice.Map()
	if choiceMap["id"] != "v_choice" || choiceMap["type"] != VARIABLE_CHOICE {
		t.Errorf("OptimizationChoice Map() serialization failed")
	}
}

func TestGetValue(t *testing.T) {
	app := &MockApplication{}
	variables := []any{
		NewOptimizationChoice("choice_var", []any{dummyFunction}),
		NewOptimizationInteger("int_var", 0, 5),
		NewOptimizationReal("float_var", 0.0, 1.0),
		NewOptimizationBinary("bool_var"),
	}

	opt := NewOptimization(variables, app, "127.0.0.1", 10000, 11000)
	
	// Set mock VariableValues
	opt.VariableValues = map[string]*OptimizationValue{
		"choice_var": {
			Id:   "choice_var_0",
			Type: VALUE_FUNCTION,
			Data: "mock_func",
		},
		"int_var": {
			Id:   "int_var",
			Type: VALUE_INTEGER,
			Data: float64(3),
		},
		"float_var": {
			Id:   "float_var",
			Type: VALUE_FLOAT,
			Data: 0.75,
		},
		"bool_var": {
			Id:   "bool_var",
			Type: VALUE_BOOLEAN,
			Data: true,
		},
	}
	opt.ExecutedVariableValues = make(map[string]any)

	choiceVar := opt.Variables["choice_var"].(*OptimizationChoice)
	choiceVar.Options["choice_var_0"] = &OptimizationValue{
		Id:   "choice_var_0",
		Type: VALUE_FUNCTION,
		Data: &OptimizationFunctionValue{
			Function: dummyFunction,
		},
	}

	res1 := opt.GetValue("choice_var")
	if res1 != "result" {
		t.Errorf("GetValue choice_var failed, got: %v", res1)
	}

	opt.ExecutedVariableValues["choice_var"] = "cached_result"
	resCached := opt.GetValue("choice_var")
	if resCached != "cached_result" {
		t.Errorf("GetValue caching failed, got: %v", resCached)
	}

	res2 := opt.GetValue("int_var")
	if res2.(int64) != 3 {
		t.Errorf("GetValue int_var failed, got: %v", res2)
	}

	res3 := opt.GetValue("float_var")
	if res3.(float64) != 0.75 {
		t.Errorf("GetValue float_var failed, got: %v", res3)
	}

	res4 := opt.GetValue("bool_var")
	if res4.(bool) != true {
		t.Errorf("GetValue bool_var failed, got: %v", res4)
	}
}

func TestEvaluatePrepareAndRun(t *testing.T) {
	app := &MockApplication{
		evaluateFunc: func(ctx *Optimization) *OptimizationEvaluateRunResponse {
			return &OptimizationEvaluateRunResponse{
				Objectives:            []float64{42.0},
				InequalityConstraints: []float64{0.5},
				EqualityConstraints:   []float64{},
			}
		},
	}
	opt := NewOptimization([]any{}, app, "127.0.0.1", 10000, 11000)

	prepReq := OptimizationEvaluatePrepareRequest{
		VariableValues: map[string]*OptimizationValue{
			"v1": {Id: "v1", Type: VALUE_INTEGER, Data: float64(2)},
		},
	}
	prepReqJson, _ := json.Marshal(prepReq)

	reqPrep := httptest.NewRequest(http.MethodPost, "/apis/optimizations/evaluates/prepares", bytes.NewReader(prepReqJson))
	rrPrep := httptest.NewRecorder()

	opt.EvaluatePrepare(rrPrep, reqPrep)
	if rrPrep.Code != http.StatusOK {
		t.Errorf("EvaluatePrepare status code: got %v want %v", rrPrep.Code, http.StatusOK)
	}
	if opt.VariableValues["v1"].Data.(float64) != 2 {
		t.Errorf("EvaluatePrepare variables not set correctly")
	}

	reqRun := httptest.NewRequest(http.MethodGet, "/apis/optimizations/evaluates/runs", nil)
	rrRun := httptest.NewRecorder()

	opt.EvaluateRun(rrRun, reqRun)
	if rrRun.Code != http.StatusOK {
		t.Errorf("EvaluateRun status code: got %v want %v", rrRun.Code, http.StatusOK)
	}

	var runResp OptimizationEvaluateRunResponse
	json.NewDecoder(rrRun.Body).Decode(&runResp)
	if len(runResp.Objectives) != 1 || runResp.Objectives[0] != 42.0 || runResp.InequalityConstraints[0] != 0.5 {
		t.Errorf("EvaluateRun output invalid: %+v", runResp)
	}
}

func TestPrepare(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/apis/optimizations/prepares" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		resp := PrepareResponse{
			Variables: map[string]PrepareResponseVariable{
				"v_bin": {Id: "v_bin", Type: VARIABLE_BINARY},
				"v_int": {Id: "v_int", Type: VARIABLE_INTEGER, Bounds: []float64{10, 20}},
				"v_real": {Id: "v_real", Type: VARIABLE_REAL, Bounds: []float64{1.2, 5.8}},
				"v_choice": {
					Id:   "v_choice",
					Type: VARIABLE_CHOICE,
					Options: map[string]PrepareResponseOption{
						"v_choice_0": {
							Id:   "v_choice_0",
							Type: VALUE_FUNCTION,
							Data: map[string]any{
								"error_potentiality":      0.1,
								"complexity":             0.2,
								"modularity":             0.8,
								"overall_maintainability": 0.9,
								"understandability":      0.95,
								"readability":            0.85,
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	app := &MockApplication{}
	variables := []any{
		NewOptimizationBinary("v_bin"),
		NewOptimizationInteger("v_int", 0, 5),
		NewOptimizationReal("v_real", 0.0, 1.0),
		NewOptimizationChoice("v_choice", []any{dummyFunction}),
	}

	opt := NewOptimization(variables, app, "127.0.0.1", 10000, 11000)
	opt.ServerUrl = server.URL
	opt.ClientPort = 39999

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Recovered from StartClientServer:", r)
			}
		}()
		opt.Prepare()
	}()

	time.Sleep(100 * time.Millisecond)

	binVar := opt.Variables["v_bin"].(*OptimizationBinary)
	if binVar.Id != "v_bin" || binVar.Type != VARIABLE_BINARY {
		t.Errorf("Prepare failed to update binary variable")
	}

	intVar := opt.Variables["v_int"].(*OptimizationInteger)
	if intVar.Bounds[0] != 10 || intVar.Bounds[1] != 20 {
		t.Errorf("Prepare failed to update integer variable bounds")
	}

	realVar := opt.Variables["v_real"].(*OptimizationReal)
	if realVar.Bounds[0] != 1.2 || realVar.Bounds[1] != 5.8 {
		t.Errorf("Prepare failed to update real variable bounds")
	}

	choiceVar := opt.Variables["v_choice"].(*OptimizationChoice)
	option := choiceVar.Options["v_choice_0"]
	if option == nil || option.Type != VALUE_FUNCTION {
		t.Errorf("Prepare failed to update choice variable option type")
	}
	funcVal := option.Data.(*OptimizationFunctionValue)
	if funcVal.Complexity != 0.2 || funcVal.OverallMaintainability != 0.9 {
		t.Errorf("Prepare failed to update choice variable option function metrics: %+v", funcVal)
	}
}
