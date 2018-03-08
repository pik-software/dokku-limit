package resource

import (
	"fmt"
	"strings"
	"io/ioutil"
	"runtime"
	"strconv"
	"github.com/dokku/dokku/plugins/common"
	units "github.com/docker/go-units"
	"github.com/jinzhu/copier"
	"gopkg.in/yaml.v2"
)


type Type string


const (
	// TypeMemory specifies the available memory in bytes inside a container.
	TypeMemory Type = "memory"
	TypeCPU Type = "cpu"
)

// TODO: Load defaults from globals
var defaults = Resources{}


type Resources map[Type]int64


func Defaults() Resources {
	r := Resources{}
	copier.Copy(&r, &defaults)
	return r
}


type Limits map[string]Resources


// Save limits
func (l Limits) SaveToApp(appName string) error {
	filePath := LimitFilePath(appName)
    rJson, _ := yaml.Marshal(l)
    err := ioutil.WriteFile(filePath, rJson, 0644)
    return err
}


// Returns formatted docker arguments
// TODO: Move this to the resources type
func (l Limits) DockerOptions(procName string) ([]string) {
	args := make([]string, len(l))

	limits, ok := l[procName]
	if !ok {
		return nil
	}

	for typ, limit := range limits {
		args = append(args, FormatLimitDocker(typ, limit))
	}

	return args
}


func FormatLimit(typ Type, limit int64) string {
	switch typ {
	case TypeMemory:
		return units.BytesSize(float64(limit))
	case TypeCPU:
		return fmt.Sprintf("%d%%", limit)
	default:
		return strconv.FormatInt(limit, 10)
	}
}


func FormatLimitDocker(typ Type, limit int64) string {
	switch typ {
	case TypeMemory:
		return fmt.Sprintf("--memory=%d", limit)
	case TypeCPU:
		numCPU := runtime.NumCPU()
		cpus := (float64(numCPU) / float64(100) * float64(limit))
		return fmt.Sprintf("--cpus=\"%.2g\"", cpus)
	}
	return ""
}


func ToType(s string) (Type, bool) {
	switch s {
	case string(TypeMemory):
		return TypeMemory, true
	case string(TypeCPU):
		return TypeCPU, true
	default:
		return Type(""), false
	}
}


func Parse(limits []string) (Resources) {
	resources := Resources{}
	for _, limit := range limits {
		typVal := strings.SplitN(limit, "=", 2)
		if len(typVal) != 2 {
			common.LogFail(fmt.Sprintf("invalid resource limit: %q", limit))
		}
		typ, ok := ToType(typVal[0])
		if !ok {
			common.LogFail(fmt.Sprintf("invalid resource limit type: %q", typVal))
		}
		val, err := ParseLimit(typ, typVal[1])
		if err != nil {
			common.LogFail(fmt.Sprintf("invalid resource limit value: %q", typVal[1]))
		}
		resources[typ] = val
	}
	return resources
}


func ParseLimit(typ Type, s string) (int64, error) {
	switch typ {
	case TypeMemory:
		return units.RAMInBytes(s)
	case TypeCPU:
		val, err := units.FromHumanSize(s)
		if (val > 100 || val <= 0) {
			return -1, fmt.Errorf("Invalid CPU value, should be between 1 - 100")
		}
		return val, err
	default:
		return units.FromHumanSize(s)
	}
}


func AppRoot(appName string) (appRoot string) {
	dokkuRoot := common.MustGetEnv("DOKKU_ROOT")
	appRoot = strings.Join([]string{dokkuRoot, appName}, "/")
	return appRoot
}


func LimitFilePath(appName string) (filePath string) {
	appRoot := AppRoot(appName)
	return strings.Join([]string{appRoot, "RESOURCES.yml"}, "/")
}


func LoadForApp(appName string) Limits {
	filePath := LimitFilePath(appName)

	if !common.FileExists(filePath) {
		return nil
	}

	raw, err := ioutil.ReadFile(filePath)
    if err != nil {
        common.LogFail(err.Error())
    }

	limits := Limits{}
    err = yaml.Unmarshal(raw, &limits)

    return limits
}


// Get the processes for an app
func GetAppProcs(appName string) map[string]bool {
	appRoot := AppRoot(appName)
	filePath := strings.Join([]string{appRoot, "DOKKU_SCALE"}, "/")
	procs := make(map[string]bool)

	if !common.FileExists(filePath) {
		return procs
	}

	lines, err := common.FileToSlice(filePath)
    if err != nil {
        common.LogFail(err.Error())
    }

    for _, line := range lines {
    	procs[strings.Split(line, "=")[0]] = true 
    }

    return procs
}