package autocode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/muazhari/gomacro-custom/fast"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"net/http"
	"reflect"
	"runtime"
	"strings"
)

const VARIABLE_BINARY = "OptimizationBinary"
const VARIABLE_INTEGER = "OptimizationInteger"
const VARIABLE_REAL = "OptimizationReal"
const VARIABLE_CHOICE = "OptimizationChoice"
const VALUE_FUNCTION = "OptimizationValueFunction"
const VALUE_BOOLEAN = "bool"
const VALUE_INTEGER = "int"
const VALUE_FLOAT = "float"

type OptimizationVariable struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type OptimizationBinary struct {
	*OptimizationVariable
}

func NewOptimizationBinary(name string) *OptimizationBinary {
	return &OptimizationBinary{
		OptimizationVariable: &OptimizationVariable{
			Id:   uuid.NewString(),
			Name: name,
			Type: VARIABLE_BINARY,
		},
	}
}

func (self *OptimizationBinary) Map() (output map[string]any) {
	data := map[string]any{}
	data["id"] = self.Id
	data["type"] = self.Type
	data["name"] = self.Name
	output = data
	return output
}

type OptimizationInteger struct {
	*OptimizationVariable
	Bounds [2]int64 `json:"bounds"`
}

func (self *OptimizationInteger) Map() (output map[string]any) {
	data := map[string]any{}
	data["id"] = self.Id
	data["type"] = self.Type
	data["name"] = self.Name
	data["bounds"] = self.Bounds
	output = data
	return output
}

func NewOptimizationInteger(name string, lowerBound int64, upperBound int64) *OptimizationInteger {
	return &OptimizationInteger{
		OptimizationVariable: &OptimizationVariable{
			Id:   uuid.NewString(),
			Name: name,
			Type: VARIABLE_INTEGER,
		},
		Bounds: [2]int64{lowerBound, upperBound},
	}
}

type OptimizationReal struct {
	*OptimizationVariable
	Bounds [2]float64 `json:"bounds"`
}

func (self *OptimizationReal) Map() (output map[string]any) {
	data := map[string]any{}
	data["id"] = self.Id
	data["type"] = self.Type
	data["name"] = self.Name
	data["bounds"] = self.Bounds
	output = data
	return output
}

func NewOptimizationReal(name string, lowerBound float64, upperBound float64) *OptimizationReal {
	return &OptimizationReal{
		OptimizationVariable: &OptimizationVariable{
			Id:   uuid.NewString(),
			Name: name,
			Type: VARIABLE_REAL,
		},
		Bounds: [2]float64{lowerBound, upperBound},
	}
}

type OptimizationChoice struct {
	*OptimizationVariable
	Options map[string]*OptimizationValue `json:"options"`
}

func (self *OptimizationChoice) Map() (output map[string]any) {
	data := map[string]any{}
	data["id"] = self.Id
	data["type"] = self.Type
	data["name"] = self.Name
	options := map[string]any{}
	data["options"] = options
	for optionId, option := range self.Options {
		options[optionId] = option.Map()
	}
	output = data
	return output

}

func getType(value any) string {
	switch value.(type) {
	case *OptimizationBinary:
		return VARIABLE_BINARY
	case *OptimizationInteger:
		return VARIABLE_INTEGER
	case *OptimizationReal:
		return VARIABLE_REAL
	case *OptimizationChoice:
		return VARIABLE_CHOICE
	case int64:
		return VALUE_INTEGER
	case float64:
		return VALUE_FLOAT
	case bool:
		return VALUE_BOOLEAN
	case FunctionValue:
		return VALUE_FUNCTION
	default:
		panic("Unknown type")
	}
}

func NewOptimizationChoice(name string, options []any) *OptimizationChoice {
	transformedOptions := map[string]*OptimizationValue{}
	for _, option := range options {
		optionId := uuid.NewString()
		optionType := getType(option)
		if optionType == VALUE_FUNCTION {
			option = &OptimizationFunctionValue{
				Function:               option.(FunctionValue),
				Complexity:             0,
				ErrorPotentiality:      0,
				Modularity:             0,
				OverallMaintainability: 0,
				Understandability:      0,
			}
		}
		transformedOptions[optionId] = &OptimizationValue{
			Id:   optionId,
			Type: optionType,
			Data: option,
		}

	}
	return &OptimizationChoice{
		OptimizationVariable: &OptimizationVariable{
			Id:   uuid.NewString(),
			Name: name,
			Type: VARIABLE_CHOICE,
		},
		Options: transformedOptions,
	}
}

type OptimizationValue struct {
	Id   string `json:"id"`
	Type string `json:"type"`
	Data any    `json:"data"`
}

func (self *OptimizationValue) Map() (output map[string]any) {
	data := map[string]any{}
	data["id"] = self.Id
	data["type"] = self.Type
	data["data"] = self.Data
	if self.Data != nil {
		if data["type"] == VALUE_FUNCTION {
			data["data"] = (self.Data.(*OptimizationFunctionValue)).Map()
		}
	}
	output = data
	return output
}

type FunctionValue = func(*Optimization, ...any) any
type OptimizationFunctionValue struct {
	Function               FunctionValue
	ErrorPotentiality      float64
	Understandability      float64
	Complexity             float64
	OverallMaintainability float64
	Modularity             float64
	Readability            float64
}

func (self *OptimizationFunctionValue) GetName() (output string) {
	output = runtime.FuncForPC(reflect.ValueOf(self.Function).Pointer()).Name()
	return output
}

func (self *OptimizationFunctionValue) Parse() (functionDeclaration *ast.FuncDecl, fileSet *token.FileSet) {
	fileSet = token.NewFileSet()
	function := runtime.FuncForPC(reflect.ValueOf(self.Function).Pointer())
	segments := strings.Split(function.Name(), ".")
	functionName := segments[len(segments)-1]
	fileName, line := function.FileLine(0)
	if file, err := parser.ParseFile(fileSet, fileName, nil, 0); err == nil {
		for _, declaration := range file.Decls {
			f, ok := declaration.(*ast.FuncDecl)
			if ok && f.Name.Name == functionName {
				functionDeclaration = f
				return functionDeclaration, fileSet
			}
		}
	}
	panic(fmt.Errorf("function not found: %s at %s:%d", functionName, fileName, line))
}

func (self *OptimizationFunctionValue) GetString() (output string) {
	functionDeclaration, fileSet := self.Parse()
	buffer := &bytes.Buffer{}
	printErr := printer.Fprint(buffer, fileSet, functionDeclaration)
	if printErr != nil {
		panic(printErr)
	}
	output = buffer.String()
	return output
}

func (self *OptimizationFunctionValue) Map() (output map[string]any) {
	data := map[string]any{}
	data["name"] = self.GetName()
	data["string"] = self.GetString()
	output = data
	return output
}

type OptimizationEvaluateRunResponse struct {
	Objectives            []float64 `json:"objectives"`
	InequalityConstraints []float64 `json:"inequality_constraints"`
	EqualityConstraints   []float64 `json:"equality_constraints"`
}

type OptimizationApplication interface {
	Duplicate(ctx *Optimization) any
	Evaluate(ctx *Optimization) *OptimizationEvaluateRunResponse
}

func (self *Optimization) GetValue(variableName string, arguments ...any) (output any) {
	variableId := ""
	for currentVariableId, currentVariable := range self.Variables {
		currentVariableName := getFieldValue(currentVariable, "Name")
		if currentVariableName == variableName {
			variableId = currentVariableId
			break
		}
	}
	if variableId == "" {
		panic(fmt.Sprintf("variable not found: %s", variableName))
	}

	executedValue, executedValueExists := self.ExecutedVariableValues[variableId]
	if executedValueExists == true {
		return executedValue
	}
	value, valueExists := self.VariableValues[variableId]
	if valueExists == false {
		panic(fmt.Sprintf("variable value not found: %s", variableName))
	}
	if value.Type == VALUE_FUNCTION {
		variable := self.Variables[variableId]
		choice := variable.(*OptimizationChoice)
		option := choice.Options[value.Id]
		function := option.Data.(*OptimizationFunctionValue)
		output = function.Function(self, arguments...)
	} else if value.Type == VALUE_INTEGER {
		output = int64(value.Data.(float64))
	} else if value.Type == VALUE_FLOAT {
		output = value.Data.(float64)
	} else if value.Type == VALUE_BOOLEAN {
		output = value.Data.(bool)
	} else {
		panic(fmt.Sprintf("unsupported value type: %s", value.Type))
	}
	self.ExecutedVariableValues[variableId] = output
	return output
}

type Optimization struct {
	Variables              map[string]any
	Application            OptimizationApplication
	ServerHost             string
	ServerPort             int64
	ClientHost             string
	ClientPort             int64
	ServerUrl              string
	ClientUrl              string
	ClientName             string
	Interpreter            *fast.Interp
	VariableValues         map[string]*OptimizationValue
	ExecutedVariableValues map[string]any
}

func NewOptimization(
	variables []any,
	application OptimizationApplication,
	serverHost string,
	serverPort int64,
	clientHost string,
	clientPort int64,
	clientName string,
	buildArgs []string,
) (optimization *Optimization) {
	transformedVariables := map[string]any{}
	for _, variable := range variables {
		variableId := getFieldValue(variable, "Id").(string)
		transformedVariables[variableId] = variable
	}
	interpreter := fast.New()
	interpreter.SetBuildArgs(buildArgs)
	imports := `
				import (
					"github.com/muazhari/autocode-go"
				)
				`
	interpreter.Eval(imports)
	optimization = &Optimization{
		Variables:   transformedVariables,
		Application: application,
		ServerHost:  serverHost,
		ServerPort:  serverPort,
		ServerUrl:   fmt.Sprintf("http://%s:%d", serverHost, serverPort),
		ClientHost:  clientHost,
		ClientPort:  clientPort,
		ClientUrl:   fmt.Sprintf("http://%s:%d", clientHost, clientPort),
		ClientName:  clientName,
		Interpreter: interpreter,
	}

	return optimization
}

func getFieldValue(variable any, field string) (output any) {
	reflectedVariable := reflect.Indirect(reflect.ValueOf(variable))
	fieldValue := reflectedVariable.FieldByName(field)
	output = fieldValue.Interface()
	return output
}

func (self *Optimization) Prepare() {
	requestBody := &OptimizationPrepareRequest{
		Variables: self.Variables,
		Host:      self.ClientHost,
		Port:      self.ClientPort,
		Name:      self.ClientName,
	}

	requestBodyMap := requestBody.Map()
	requestBodyJson, jsonErr := json.Marshal(requestBodyMap)
	if jsonErr != nil {
		panic(jsonErr)
	}
	bodyBuffer := bytes.NewBuffer(requestBodyJson)
	client := &http.Client{
		Timeout: 0,
	}
	url := fmt.Sprintf("%s/apis/optimizations/prepares", self.ServerUrl)
	response, responseErr := client.Post(url, "application/json", bodyBuffer)
	if responseErr != nil {
		panic(responseErr)
	}

	if response.StatusCode != 200 {
		panic("Failed to prepare")
	}

	responseBody := &OptimizationPrepareResponse{}
	decodeErr := json.NewDecoder(response.Body).Decode(responseBody)
	if decodeErr != nil {
		panic(decodeErr)
	}

	for newVariableId, newVariable := range responseBody.Variables {
		newVariableType := newVariable.(map[string]any)["type"].(string)
		newVariableName := newVariable.(map[string]any)["name"].(string)
		oldVariable, _ := self.Variables[newVariableId]
		if newVariableType == VARIABLE_CHOICE {
			oldOptions := getFieldValue(oldVariable, "Options").(map[string]*OptimizationValue)
			newOptions := map[string]*OptimizationValue{}
			for optionId, option := range newVariable.(map[string]any)["options"].(map[string]any) {
				oldOption, isExists := oldOptions[optionId]
				optionType := option.(map[string]any)["type"].(string)
				optionData := option.(map[string]any)["data"].(map[string]any)
				if optionType == VALUE_FUNCTION {
					if isExists == true {
						oldOptionData := oldOption.Data.(*OptimizationFunctionValue)
						oldOptionData.ErrorPotentiality = optionData["error_potentiality"].(float64)
						oldOptionData.Complexity = optionData["complexity"].(float64)
						oldOptionData.Modularity = optionData["modularity"].(float64)
						oldOptionData.OverallMaintainability = optionData["overall_maintainability"].(float64)
						oldOptionData.Understandability = optionData["understandability"].(float64)
						oldOptionData.Readability = optionData["readability"].(float64)
						newOptions[optionId] = oldOption
					} else {
						splitFunctionName := strings.Split(optionData["name"].(string), ".")
						functionName := splitFunctionName[len(splitFunctionName)-1]
						functionString := optionData["string"].(string)
						self.Interpreter.Eval(functionString)
						function, _ := self.Interpreter.Eval1(functionName)
						newOptionData := &OptimizationFunctionValue{
							Function:               function.Interface().(FunctionValue),
							ErrorPotentiality:      optionData["error_potentiality"].(float64),
							Complexity:             optionData["complexity"].(float64),
							Modularity:             optionData["modularity"].(float64),
							OverallMaintainability: optionData["overall_maintainability"].(float64),
							Understandability:      optionData["understandability"].(float64),
							Readability:            optionData["readability"].(float64),
						}
						newOptions[optionId] = &OptimizationValue{
							Id:   optionId,
							Type: optionType,
							Data: newOptionData,
						}
					}
				} else {
					newOptions[optionId] = oldOption
				}
			}
			newChoice := &OptimizationChoice{
				OptimizationVariable: &OptimizationVariable{
					Id:   newVariableId,
					Type: VARIABLE_CHOICE,
					Name: newVariableName,
				},
				Options: newOptions,
			}
			self.Variables[newVariableId] = newChoice
		}
	}

	self.StartClientServer()
}

func (self *Optimization) StartClientServer() {
	router := mux.NewRouter()
	apiRouter := router.PathPrefix("/apis").Subrouter()
	apiRouter.HandleFunc("/optimizations/evaluates/prepares", self.EvaluatePrepare).Methods(http.MethodPost)
	apiRouter.HandleFunc("/optimizations/evaluates/runs", self.EvaluateRun).Methods(http.MethodGet)
	address := fmt.Sprintf("%s:%d", self.ClientHost, self.ClientPort)
	serverErr := fasthttp.ListenAndServe(address, fasthttpadaptor.NewFastHTTPHandler(router))
	if serverErr != nil {
		panic(serverErr)
	}
}

func (self *Optimization) EvaluatePrepare(writer http.ResponseWriter, reader *http.Request) {
	requestBody := &OptimizationEvaluatePrepareRequest{}
	decodeErr := json.NewDecoder(reader.Body).Decode(requestBody)
	if decodeErr != nil {
		panic(decodeErr)
	}

	self.VariableValues = requestBody.VariableValues
	self.ExecutedVariableValues = map[string]any{}
}

func (self *Optimization) EvaluateRun(writer http.ResponseWriter, reader *http.Request) {
	evaluation := self.Application.Evaluate(self)

	encodeErr := json.NewEncoder(writer).Encode(evaluation)
	if encodeErr != nil {
		panic(encodeErr)
	}
}

type OptimizationPrepareRequest struct {
	Variables map[string]any `json:"variables"`
	Host      string         `json:"host"`
	Port      int64          `json:"port"`
	Name      string         `json:"name"`
}

func (self *OptimizationPrepareRequest) Map() map[string]any {
	transformedVariables := map[string]any{}
	for variableId, variable := range self.Variables {
		variableType := getType(variable)
		switch variableType {
		case VARIABLE_BINARY:
			transformedVariables[variableId] = variable.(*OptimizationBinary).Map()
		case VARIABLE_INTEGER:
			transformedVariables[variableId] = variable.(*OptimizationInteger).Map()
		case VARIABLE_REAL:
			transformedVariables[variableId] = variable.(*OptimizationReal).Map()
		case VARIABLE_CHOICE:
			transformedVariables[variableId] = variable.(*OptimizationChoice).Map()
		default:
			panic("Unknown type")
		}
	}
	return map[string]any{
		"variables": transformedVariables,
		"host":      self.Host,
		"port":      self.Port,
		"name":      self.Name,
	}
}

type OptimizationPrepareResponse struct {
	Variables map[string]any `json:"variables"`
}

type OptimizationEvaluatePrepareRequest struct {
	VariableValues map[string]*OptimizationValue `json:"variable_values"`
}

type OptimizationEvaluateRunRequest struct {
}
