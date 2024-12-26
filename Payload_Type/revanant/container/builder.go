package agentfunctions

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
	"github.com/pelletier/go-toml/v2"
)

const version = "1.0.0"

type sleepInfoStruct struct {
	Interval int       `json:"interval"`
	Jitter   int       `json:"jitter"`
	KillDate time.Time `json:"killdate"`
}

var payloadDefinition = agentstructs.PayloadType{
	Name:                                   "revanant",
	FileExtension:                          ".bin",
	Author:                                 "@xcr-19",
	SupportedOS:                            []string{agentstructs.SUPPORTED_OS_WINDOWS},
	Wrapper:                                false,
	CanBeWrappedByTheFollowingPayloadTypes: []string{},
	SupportsDynamicLoading:                 false,
	Description:                            fmt.Sprintf("Revanant is a work in progress Mythic agent for Windows, Its a hobby project to learn go"),
	SupportedC2Profiles:                    []string{"http"},
	MythicEncryptsData:                     true,
	BuildParameters: []agentstructs.BuildParameter{
		{
			Name:          "mode",
			Description:   "Choose the build mode option. Select default for executables, c-shared for a .dylib or .so file, or c-archive for a .Zip containing C source code with an archive and header file",
			Required:      false,
			DefaultValue:  "default",
			Choices:       []string{"default", "c-archive", "c-shared"},
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_CHOOSE_ONE,
		},
		{
			Name:          "architecture",
			Description:   "Choose the agent's architecture",
			Required:      false,
			DefaultValue:  "AMD_x64",
			Choices:       []string{"AMD_x64", "ARM_x64"},
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_CHOOSE_ONE,
		},
		{
			Name:          "proxy_bypass",
			Description:   "Ignore HTTP proxy environment settings configured on the target host?",
			Required:      false,
			DefaultValue:  false,
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_BOOLEAN,
		},
		{
			Name:          "garble",
			Description:   "Use Garble to obfuscate the output Go executable.\nWARNING - This significantly slows the agent build time.",
			Required:      false,
			DefaultValue:  false,
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_BOOLEAN,
		},
		{
			Name:          "debug",
			Description:   "Create a debug build with print statements for debugging.",
			Required:      false,
			DefaultValue:  false,
			ParameterType: agentstructs.BUILD_PARAMETER_TYPE_BOOLEAN,
		},
	},
	BuildSteps: []agentstructs.BuildStep{
		{
			Name:        "Configuring",
			Description: "Cleaning up configuration values and generating the golang build command",
		},
		{
			Name:        "Garble",
			Description: "Adding in Garble (obfuscation)",
		},
		{
			Name:        "Compiling",
			Description: "Compiling the golang agent",
		},
	},
	CheckIfCallbacksAliveFunction: func(message agentstructs.PTCheckIfCallbacksAliveMessage) agentstructs.PTCheckIfCallbacksAliveMessageResponse {
		response := agentstructs.PTCheckIfCallbacksAliveMessageResponse{Success: true, Callbacks: make([]agentstructs.PTCallbacksToCheckResponse, 0)}
		for _, callback := range message.Callbacks {
			//logging.LogInfo("callback info", "callback", callback)
			if callback.SleepInfo == "" {
				continue // can't do anything if we don't know the expected sleep info of the agent
			}
			sleepInfo := map[string]sleepInfoStruct{}
			err := json.Unmarshal([]byte(callback.SleepInfo), &sleepInfo)
			if err != nil {
				//logging.LogError(err, "failed to parse sleep info struct")
				continue
			}
			atLeastOneCallbackWithinRange := false
			for activeC2, _ := range sleepInfo {
				if activeC2 == "websocket" && callback.LastCheckin.Unix() == 0 {
					atLeastOneCallbackWithinRange = true
					continue
				}
				if activeC2 == "poseidon_tcp" {
					atLeastOneCallbackWithinRange = true
					continue
				}
				minAdd := sleepInfo[activeC2].Interval
				maxAdd := sleepInfo[activeC2].Interval
				if sleepInfo[activeC2].Jitter > 0 {
					// minimum would be sleep_interval - (sleep_jitter % of sleep_interval)
					minAdd = minAdd - ((sleepInfo[activeC2].Jitter / 100) * (sleepInfo[activeC2].Interval))
					// maximum would be sleep_interval + (sleep_jitter % of sleep_interval)
					maxAdd = maxAdd + ((sleepInfo[activeC2].Jitter / 100) * (sleepInfo[activeC2].Interval))
				}
				maxAdd *= 2 // double the high end in case we're on a close boundary
				earliest := callback.LastCheckin.Add(time.Duration(minAdd) * time.Second)
				latest := callback.LastCheckin.Add(time.Duration(maxAdd) * time.Second)

				if callback.LastCheckin.After(earliest) && callback.LastCheckin.Before(latest) {
					atLeastOneCallbackWithinRange = true
				}
			}
			response.Callbacks = append(response.Callbacks, agentstructs.PTCallbacksToCheckResponse{
				ID:    callback.ID,
				Alive: atLeastOneCallbackWithinRange,
			})
		}
		return response
	},
}

func build(payloadBuildMsg agentstructs.PayloadBuildMessage) agentstructs.PayloadBuildResponse {
	payloadBuildResponse := agentstructs.PayloadBuildResponse{
		PayloadUUID:        payloadBuildMsg.PayloadUUID,
		Success:            true,
		UpdatedCommandList: &payloadBuildMsg.CommandList,
	}
	if len(payloadBuildMsg.C2Profiles) == 0 {
		payloadBuildResponse.Success = false
		payloadBuildResponse.BuildStdErr = "Failed to build - must select at least one C2 profile"
		return payloadBuildResponse
	}
	targetOS := "windows"
	debug, err := payloadBuildMsg.BuildParameters.GetBooleanArg("debug")
	if err != nil {
		payloadBuildResponse.Success = false
		payloadBuildResponse.BuildStdErr = err.Error()
		return payloadBuildResponse
	}

	revanant_repo_profile := "github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/profiles"
	revanant_repo_utils := "github.com/xcr-19/revanant/Payload_Type/revanant/agent/pkg/utils"

	ldflags := ""
	ldflags += fmt.Sprintf("-s -w -X '%s.UUID=%s'", revanant_repo_profile, payloadBuildMsg.PayloadUUID)
	ldflags += fmt.Sprintf(" -X '%s.debugString=%v'", revanant_repo_profile, debug)

	for index, _ := range payloadBuildMsg.C2Profiles {
		initialConfig := make(map[string]interface{})
		for _, key := range payloadBuildMsg.C2Profiles[index].GetArgNames() {
			if key == "AESPSK" {
				cryptoVal, err := payloadBuildMsg.C2Profiles[index].GetCryptoArg(key)
				if err != nil {
					payloadBuildResponse.Success = false
					payloadBuildResponse.BuildStdErr = "Key error: " + key + "\n" + err.Error()
					return payloadBuildResponse
				}
				initialConfig[key] = cryptoVal.EncKey
			} else if key == "headers" {
				headers, err := payloadBuildMsg.C2Profiles[index].GetDictionaryArg(key)
				if err != nil {
					payloadBuildResponse.Success = false
					payloadBuildResponse.BuildStdErr = "Key error: " + key + "\n" + err.Error()
					return payloadBuildResponse
				}
				initialConfig[key] = headers
			} else if key == "raw_c2_config" {
				agentConfigString, err := payloadBuildMsg.C2Profiles[index].GetStringArg(key)
				if err != nil {
					payloadBuildResponse.Success = false
					payloadBuildResponse.BuildStdErr = "Key error: " + key + "\n" + err.Error()
					return payloadBuildResponse
				}
				configData, err := mythicrpc.SendMythicRPCFileGetContent(mythicrpc.MythicRPCFileGetContentMessage{
					AgentFileID: agentConfigString,
				})
				if err != nil {
					payloadBuildResponse.Success = false
					payloadBuildResponse.BuildStdErr = "Key error: " + key + "\n" + err.Error()
					return payloadBuildResponse
				}
				if !configData.Success {
					payloadBuildResponse.Success = false
					payloadBuildResponse.BuildStdErr = "Key error: " + key + "\n" + configData.Error
					return payloadBuildResponse
				}
				tomlConfig := make(map[string]interface{})
				err = json.Unmarshal(configData.Content, &tomlConfig)
				if err != nil {
					err = toml.Unmarshal(configData.Content, &tomlConfig)
					if err != nil {
						payloadBuildResponse.Success = false
						payloadBuildResponse.BuildStdErr = "Key error: " + key + "\n" + err.Error()
						return payloadBuildResponse
					}
				}
				initialConfig[key] = tomlConfig
			}
		}
		initialConfigByptes, err := json.Marshal(initialConfig)
		if err != nil {
			payloadBuildResponse.Success = false
			payloadBuildResponse.BuildStdErr = err.Error()
			return payloadBuildResponse
		}
		initialConfigBase64 := base64.StdEncoding.EncodeToString(initialConfigByptes)
		payloadBuildResponse.BuildStdOut += fmt.Sprintf("%s's config: \n%v\n", payloadBuildMsg.C2Profiles[index].Name, string(initialConfigByptes))
		ldflags += fmt.Sprintf(" -X '%s.%s_%s=%v'", revanant_repo_profile, payloadBuildMsg.C2Profiles[index].Name, "initial_config", initialConfigBase64)
	}

	architecture, err := payloadBuildMsg.BuildParameters.GetStringArg("architecture")
	if err != nil {
		payloadBuildResponse.Success = false
		payloadBuildResponse.BuildStdErr = err.Error()
		return payloadBuildResponse
	}

	ldflags += " -buildid="
	goarch := "amd64"
	tags := []strings{}
	for index, _ := range payloadBuildMsg.C2Profiles {
		tags = append(tags, payloadBuildMsg.C2Profiles[index].Name)
	}
	command := fmt.Sprintf("CGO_ENABLED=1 GOOS=%s GOARCH=%s ", targetOS, goarch)
	goCmd := fmt.Sprintf("-tags %s -buildmode %s -ldflags \"%s\"", strings.Join(tags, ","), mode, ldflags)
	command += "CC=x86_64-w64-mingw32-gcc"
	command += "GOGARBLE=*"
	command += "go build "
	payloadName := fmt.Sprintf("%s-%s", payloadBuildMsg.PayloadUUID, targetOS)
	command += fmt.Sprintf("%s -o /build/%s", goCmd, payloadName)
	command += fmt.Sprintf("-%s", goarch)
	paylodName += fmt.Sprintf("-%s", goarch)

	mythicrpc.SendMythicRPCPayloadUpdateBuildStep(mythicrpc.MythicRPCPayloadUpdateBuildStepMessage{
		PayloadUUID: payloadBuildMsg.PayloadUUID,
		StepName:    "Configuring",
		StepSuccess: true,
		StepStdout:  fmt.Sprintf("Successfully configured \n%s", command),
	})

	cmd := exec.Command("/bin/bash")
	cmd.Stdin = strings.NewReader(command)
	cmd.Dir = "./revanant/agent/"
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		payloadBuildResponse.Success = false
		payloadBuildResponse.BuildMessage = "Compile failed with errors"
		payloadBuildResponse.BuildStdErr += stderr.String() + "\n" + err.Error()
		payloadBuildResponse.BuildStdOut += stdout.String()
	}
}
