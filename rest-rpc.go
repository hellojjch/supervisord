package main

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/ochinchina/supervisord/config"
	"github.com/ochinchina/supervisord/types"
)

// SupervisorRestful the restful interface to control the programs defined in configuration file
type SupervisorRestful struct {
	router     *mux.Router
	supervisor *Supervisor
}

// NewSupervisorRestful create a new SupervisorRestful object
func NewSupervisorRestful(supervisor *Supervisor) *SupervisorRestful {
	return &SupervisorRestful{router: mux.NewRouter(), supervisor: supervisor}
}

// CreateProgramHandler create http handler to process program related restful request
func (sr *SupervisorRestful) CreateProgramHandler() http.Handler {
	sr.router.HandleFunc("/program/list", sr.ListProgram).Methods("GET")
	sr.router.HandleFunc("/program/info/{name}", sr.GetProgramInfo).Methods("GET")
	sr.router.HandleFunc("/program/add", sr.AddProgram).Methods("POST")
	sr.router.HandleFunc("/program/copy/{name}", sr.CopyProgram).Methods("POST")
	sr.router.HandleFunc("/program/update/{name}", sr.UpdateProgram).Methods("PUT")
	sr.router.HandleFunc("/program/delete/{name}", sr.DeleteProgram).Methods("DELETE")
	sr.router.HandleFunc("/program/start/{name}", sr.StartProgram).Methods("POST", "PUT")
	sr.router.HandleFunc("/program/stop/{name}", sr.StopProgram).Methods("POST", "PUT")
	sr.router.HandleFunc("/program/log/{name}/stdout", sr.ReadStdoutLog).Methods("GET")
	sr.router.HandleFunc("/program/startPrograms", sr.StartPrograms).Methods("POST", "PUT")
	sr.router.HandleFunc("/program/stopPrograms", sr.StopPrograms).Methods("POST", "PUT")
	return sr.router
}

// CreateSupervisorHandler create http rest interface to control supervisor itself
func (sr *SupervisorRestful) CreateSupervisorHandler() http.Handler {
	sr.router.HandleFunc("/shutdown", sr.Shutdown).Methods("PUT", "POST")
	sr.router.HandleFunc("/reload", sr.Reload).Methods("PUT", "POST")
	sr.router.HandleFunc("/login", sr.Login).Methods("POST")
	
	// Nacos配置相关API 
	sr.router.HandleFunc("/supervisor/nacos/config", sr.GetNacosConfig).Methods("GET")
	sr.router.HandleFunc("/supervisor/nacos/config", sr.SaveNacosConfig).Methods("POST")
	sr.router.HandleFunc("/supervisor/nacos/test", sr.TestNacosConnection).Methods("POST")
	
	return sr.router
}

// Login handles user authentication
func (sr *SupervisorRestful) Login(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	
	// 解析请求体
	var loginData struct {
		Username   string `json:"username"`
		Password   string `json:"password"`
		RememberMe bool   `json:"remember_me"`
	}
	
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&loginData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的请求数据", "message": "请求格式不正确"})
		return
	}
	
	// 验证用户名和密码
	// 这里使用简单的硬编码验证，实际应用中应该使用更安全的方式
	if loginData.Username == "admin" && loginData.Password == "admin123" {
		// 登录成功，设置Cookie
		expiration := time.Now().Add(24 * time.Hour)
		if loginData.RememberMe {
			expiration = time.Now().Add(30 * 24 * time.Hour) // 30天
		}
		
		// 创建一个简单的会话令牌
		sessionToken := "session_" + strconv.FormatInt(time.Now().Unix(), 10)
		
		// 设置Cookie
		cookie := http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Expires:  expiration,
			HttpOnly: true,
			Path:     "/",
		}
		http.SetCookie(w, &cookie)
		
		// 返回成功
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "登录成功",
			"user": map[string]string{
				"username": loginData.Username,
				"role": "admin",
			},
		})
	} else {
		// 登录失败
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "认证失败", "message": "用户名或密码不正确"})
	}
}

// ListProgram list the status of all the programs
//
// json array to present the status of all programs
func (sr *SupervisorRestful) ListProgram(w http.ResponseWriter, req *http.Request) {
	result := struct{ AllProcessInfo []types.ProcessInfo }{make([]types.ProcessInfo, 0)}
	if sr.supervisor.GetAllProcessInfo(nil, nil, &result) == nil {
		json.NewEncoder(w).Encode(result.AllProcessInfo)
	} else {
		r := map[string]bool{"success": false}
		json.NewEncoder(w).Encode(r)
	}
}

// StartProgram start the given program through restful interface
func (sr *SupervisorRestful) StartProgram(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	params := mux.Vars(req)
	success, err := sr._startProgram(params["name"])
	r := map[string]bool{"success": err == nil && success}
	json.NewEncoder(w).Encode(&r)
}

func (sr *SupervisorRestful) _startProgram(program string) (bool, error) {
	startArgs := StartProcessArgs{Name: program, Wait: true}
	result := struct{ Success bool }{false}
	err := sr.supervisor.StartProcess(nil, &startArgs, &result)
	return result.Success, err
}

// StartPrograms start one or more programs through restful interface
func (sr *SupervisorRestful) StartPrograms(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	var b []byte
	var err error

	if b, err = ioutil.ReadAll(req.Body); err != nil {
		w.WriteHeader(400)
		w.Write([]byte("not a valid request"))
		return
	}

	var programs []string
	if err = json.Unmarshal(b, &programs); err != nil {
		w.WriteHeader(400)
		w.Write([]byte("not a valid request"))
	} else {
		for _, program := range programs {
			sr._startProgram(program)
		}
		w.Write([]byte("Success to start the programs"))
	}
}

// StopProgram stop a program through the restful interface
func (sr *SupervisorRestful) StopProgram(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	params := mux.Vars(req)
	success, err := sr._stopProgram(params["name"])
	r := map[string]bool{"success": err == nil && success}
	json.NewEncoder(w).Encode(&r)
}

func (sr *SupervisorRestful) _stopProgram(programName string) (bool, error) {
	stopArgs := StartProcessArgs{Name: programName, Wait: true}
	result := struct{ Success bool }{false}
	err := sr.supervisor.StopProcess(nil, &stopArgs, &result)
	return result.Success, err
}

// StopPrograms stop programs through the restful interface
func (sr *SupervisorRestful) StopPrograms(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	var programs []string
	var b []byte
	var err error
	if b, err = ioutil.ReadAll(req.Body); err != nil {
		w.WriteHeader(400)
		w.Write([]byte("not a valid request"))
		return
	}

	if err := json.Unmarshal(b, &programs); err != nil {
		w.WriteHeader(400)
		w.Write([]byte("not a valid request"))
	} else {
		for _, program := range programs {
			sr._stopProgram(program)
		}
		w.Write([]byte("Success to stop the programs"))
	}

}

// GetProgramInfo get the detailed information of a program
func (sr *SupervisorRestful) GetProgramInfo(w http.ResponseWriter, req *http.Request) {
	params := mux.Vars(req)
	programName := params["name"]
	
	// 准备参数和返回结构
	args := struct{ Name string }{Name: programName}
	reply := struct{ ProcInfo types.ProcessInfo }{}
	
	// 调用supervisor的GetProcessInfo方法
	err := sr.supervisor.GetProcessInfo(req, &args, &reply)
	
	if err == nil {
		// 成功获取程序信息，返回JSON
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(reply.ProcInfo)
	} else {
		// 获取失败，返回错误信息
		w.WriteHeader(http.StatusNotFound)
		errResp := map[string]string{"error": err.Error()}
		json.NewEncoder(w).Encode(errResp)
	}
}

// AddProgram adds a new program to the supervisor configuration file
func (sr *SupervisorRestful) AddProgram(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	
	// 解析请求体
	var programData struct {
		Name           string `json:"name"`
		Command        string `json:"command"`
		Autostart      bool   `json:"autostart"`
		Autorestart    bool   `json:"autorestart"`
		Environment    string `json:"environment"`
		Directory      string `json:"directory"`
		User           string `json:"user"`
		StdoutLogfile  string `json:"stdout_logfile"`
		StderrLogfile  string `json:"stderr_logfile"`
		ProcessName    string `json:"process_name"`
		NumProcs       int    `json:"numprocs"`
	}
	
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&programData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的请求数据: " + err.Error()})
		return
	}
	
	// 验证必填字段
	if programData.Name == "" || programData.Command == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "程序名称和命令不能为空"})
		return
	}
	
	// 构建程序配置
	programConfig := "\n[program:" + programData.Name + "]\n"
	programConfig += "command = " + programData.Command + "\n"
	programConfig += "autostart = " + strconv.FormatBool(programData.Autostart) + "\n"
	programConfig += "autorestart = " + strconv.FormatBool(programData.Autorestart) + "\n"
	
	if programData.Environment != "" {
		programConfig += "environment = " + programData.Environment + "\n"
	}
	if programData.Directory != "" {
		programConfig += "directory = " + programData.Directory + "\n"
	}
	if programData.User != "" {
		programConfig += "user = " + programData.User + "\n"
	}
	if programData.StdoutLogfile != "" {
		programConfig += "stdout_logfile = " + programData.StdoutLogfile + "\n"
	}
	if programData.StderrLogfile != "" {
		programConfig += "stderr_logfile = " + programData.StderrLogfile + "\n"
	}
	
	// 添加进程名称格式和进程数量
	if programData.ProcessName != "" {
		programConfig += "process_name = " + programData.ProcessName + "\n"
	}
	if programData.NumProcs > 0 {
		programConfig += "numprocs = " + strconv.Itoa(programData.NumProcs) + "\n"
	}
	
	// 检查是否使用Nacos配置
	nacosProvider, isNacosProvider := sr.supervisor.GetConfig().GetProvider().(*config.NacosConfigProvider)
	if isNacosProvider {
		// 使用Nacos配置时，直接修改Nacos中的配置
		
		// 获取当前配置
		myini, err := nacosProvider.GetConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "获取Nacos配置失败: " + err.Error()})
			return
		}
		
		// 检查程序是否已存在
		if myini.HasSection("program:" + programData.Name) {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "程序 " + programData.Name + " 已存在"})
			return
		}
		
		// 将配置转换为字符串
		configStr := myini.String()
		
		// 添加新程序配置
		newContent := configStr + programConfig
		
		// 保存到Nacos
		err = nacosProvider.SaveConfig(newContent)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "保存Nacos配置失败: " + err.Error()})
			return
		}
	} else {
		// 使用本地文件配置
		
		// 读取现有的supervisor.conf文件
		configFile := sr.supervisor.GetConfig().GetConfigFileDir() + "/supervisor.conf"
		content, err := ioutil.ReadFile(configFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "读取配置文件失败: " + err.Error()})
			return
		}
		
		// 检查程序是否已存在
		if strings.Contains(string(content), "[program:"+programData.Name+"]") {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "程序 " + programData.Name + " 已存在"})
			return
		}
		
		// 将新配置追加到文件
		newContent := string(content) + programConfig
		err = ioutil.WriteFile(configFile, []byte(newContent), 0644)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "写入配置文件失败: " + err.Error()})
			return
		}
	}
	
	// 重新加载配置
	_, _, _, err := sr.supervisor.Reload(false)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "重新加载配置失败: " + err.Error()})
		return
	}
	
	// 返回成功
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "程序 " + programData.Name + " 已成功添加",
		"program": programData,
	})
}

// UpdateProgram updates an existing program in the supervisor configuration file
func (sr *SupervisorRestful) UpdateProgram(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	
	// 获取程序名称
	params := mux.Vars(req)
	programName := params["name"]
	
	// 解析请求体
	var programData struct {
		Name           string `json:"name"`
		Command        string `json:"command"`
		Autostart      bool   `json:"autostart"`
		Autorestart    bool   `json:"autorestart"`
		Environment    string `json:"environment"`
		Directory      string `json:"directory"`
		User           string `json:"user"`
		StdoutLogfile  string `json:"stdout_logfile"`
		StderrLogfile  string `json:"stderr_logfile"`
		ProcessName    string `json:"process_name"`
		NumProcs       int    `json:"numprocs"`
	}
	
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&programData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的请求数据: " + err.Error()})
		return
	}
	
	// 验证必填字段
	if programData.Command == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "命令不能为空"})
		return
	}
	
	// 构建新的程序配置
	newProgramConfig := "[program:" + programName + "]\n"
	newProgramConfig += "command = " + programData.Command + "\n"
	newProgramConfig += "autostart = " + strconv.FormatBool(programData.Autostart) + "\n"
	newProgramConfig += "autorestart = " + strconv.FormatBool(programData.Autorestart) + "\n"
	
	if programData.Environment != "" {
		newProgramConfig += "environment = " + programData.Environment + "\n"
	}
	if programData.Directory != "" {
		newProgramConfig += "directory = " + programData.Directory + "\n"
	}
	if programData.User != "" {
		newProgramConfig += "user = " + programData.User + "\n"
	}
	if programData.StdoutLogfile != "" {
		newProgramConfig += "stdout_logfile = " + programData.StdoutLogfile + "\n"
	}
	if programData.StderrLogfile != "" {
		newProgramConfig += "stderr_logfile = " + programData.StderrLogfile + "\n"
	}
	
	// 添加进程名称格式和进程数量
	if programData.ProcessName != "" {
		newProgramConfig += "process_name = " + programData.ProcessName + "\n"
	}
	if programData.NumProcs > 0 {
		newProgramConfig += "numprocs = " + strconv.Itoa(programData.NumProcs) + "\n"
	}
	
	// 检查是否使用Nacos配置
	nacosProvider, isNacosProvider := sr.supervisor.GetConfig().GetProvider().(*config.NacosConfigProvider)
	if isNacosProvider {
		// 使用Nacos配置时，直接修改Nacos中的配置
		
		// 获取当前配置
		myini, err := nacosProvider.GetConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "获取Nacos配置失败: " + err.Error()})
			return
		}
		
		// 检查程序是否存在
		if !myini.HasSection("program:" + programName) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "程序 " + programName + " 不存在"})
			return
		}
		
		// 将配置转换为字符串
		configStr := myini.String()
		
		// 查找程序配置的开始位置
		startIndex := strings.Index(configStr, "[program:"+programName+"]")
		if startIndex == -1 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "无法找到程序配置"})
			return
		}
		
		// 查找下一个配置节的开始位置或文件结束
		endIndex := len(configStr)
		nextSectionIndex := strings.Index(configStr[startIndex+1:], "[")
		if nextSectionIndex != -1 {
			endIndex = startIndex + 1 + nextSectionIndex
		}
		
		// 替换配置
		newContent := configStr[:startIndex] + newProgramConfig + configStr[endIndex:]
		
		// 保存到Nacos
		err = nacosProvider.SaveConfig(newContent)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "保存Nacos配置失败: " + err.Error()})
			return
		}
	} else {
		// 使用本地文件配置
		
		// 读取现有的supervisor.conf文件
		configFile := sr.supervisor.GetConfig().GetConfigFileDir() + "/supervisor.conf"
		content, err := ioutil.ReadFile(configFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "读取配置文件失败: " + err.Error()})
			return
		}
		
		// 检查程序是否存在
		contentStr := string(content)
		if !strings.Contains(contentStr, "[program:"+programName+"]") {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "程序 " + programName + " 不存在"})
			return
		}
		
		// 查找程序配置的开始位置
		startIndex := strings.Index(contentStr, "[program:"+programName+"]")
		if startIndex == -1 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "无法找到程序配置"})
			return
		}
		
		// 查找下一个配置节的开始位置或文件结束
		endIndex := len(contentStr)
		nextSectionIndex := strings.Index(contentStr[startIndex+1:], "[")
		if nextSectionIndex != -1 {
			endIndex = startIndex + 1 + nextSectionIndex
		}
		
		// 替换配置
		newContent := contentStr[:startIndex] + newProgramConfig + contentStr[endIndex:]
		
		// 写入文件
		err = ioutil.WriteFile(configFile, []byte(newContent), 0644)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "写入配置文件失败: " + err.Error()})
			return
		}
	}
	
	// 重新加载配置
	_, _, _, err := sr.supervisor.Reload(false)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "重新加载配置失败: " + err.Error()})
		return
	}
	
	// 返回成功
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "程序 " + programName + " 已成功更新",
		"program": programData,
	})
}

// DeleteProgram deletes a program from the supervisor configuration file
func (sr *SupervisorRestful) DeleteProgram(w http.ResponseWriter, req *http.Request) {
	// 获取程序名称
	params := mux.Vars(req)
	programName := params["name"]
	
	// 检查是否使用Nacos配置
	nacosProvider, isNacosProvider := sr.supervisor.GetConfig().GetProvider().(*config.NacosConfigProvider)
	if isNacosProvider {
		// 使用Nacos配置时，直接修改Nacos中的配置
		
		// 获取当前配置
		myini, err := nacosProvider.GetConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "获取Nacos配置失败: " + err.Error()})
			return
		}
		
		// 检查程序是否存在
		if !myini.HasSection("program:" + programName) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "程序 " + programName + " 不存在"})
			return
		}
		
		// 将配置转换为字符串
		configStr := myini.String()
		
		// 查找程序配置的开始位置
		startIndex := strings.Index(configStr, "[program:"+programName+"]")
		if startIndex == -1 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "无法找到程序配置"})
			return
		}
		
		// 查找下一个配置节的开始位置或文件结束
		endIndex := len(configStr)
		nextSectionIndex := strings.Index(configStr[startIndex+1:], "[")
		if nextSectionIndex != -1 {
			endIndex = startIndex + 1 + nextSectionIndex
		}
		
		// 删除配置
		newContent := configStr[:startIndex] + configStr[endIndex:]
		
		// 保存到Nacos
		err = nacosProvider.SaveConfig(newContent)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "保存Nacos配置失败: " + err.Error()})
			return
		}
	} else {
		// 使用本地文件配置
		
		// 读取现有的supervisor.conf文件
		configFile := sr.supervisor.GetConfig().GetConfigFileDir() + "/supervisor.conf"
		content, err := ioutil.ReadFile(configFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "读取配置文件失败: " + err.Error()})
			return
		}
		
		// 检查程序是否存在
		contentStr := string(content)
		if !strings.Contains(contentStr, "[program:"+programName+"]") {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "程序 " + programName + " 不存在"})
			return
		}
		
		// 查找程序配置的开始位置
		startIndex := strings.Index(contentStr, "[program:"+programName+"]")
		if startIndex == -1 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "无法找到程序配置"})
			return
		}
		
		// 查找下一个配置节的开始位置或文件结束
		endIndex := len(contentStr)
		nextSectionIndex := strings.Index(contentStr[startIndex+1:], "[")
		if nextSectionIndex != -1 {
			endIndex = startIndex + 1 + nextSectionIndex
		}
		
		// 删除配置
		newContent := contentStr[:startIndex] + contentStr[endIndex:]
		
		// 写入文件
		err = ioutil.WriteFile(configFile, []byte(newContent), 0644)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "写入配置文件失败: " + err.Error()})
			return
		}
	}
	
	// 重新加载配置
	_, _, _, err := sr.supervisor.Reload(false)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "重新加载配置失败: " + err.Error()})
		return
	}
	
	// 返回成功
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "程序 " + programName + " 已成功删除",
	})
}

// CopyProgram copies an existing program to create a new program
func (sr *SupervisorRestful) CopyProgram(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	
	// 获取要复制的程序名称
	params := mux.Vars(req)
	sourceProgramName := params["name"]
	
	// 解析请求体，获取新程序名称
	var copyData struct {
		NewName string `json:"new_name"`
	}
	
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&copyData); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的请求数据: " + err.Error()})
		return
	}
	
	// 验证新程序名称
	if copyData.NewName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "新程序名称不能为空"})
		return
	}
	
	// 检查是否使用Nacos配置
	nacosProvider, isNacosProvider := sr.supervisor.GetConfig().GetProvider().(*config.NacosConfigProvider)
	if isNacosProvider {
		// 使用Nacos配置时，直接修改Nacos中的配置
		
		// 获取当前配置
		myini, err := nacosProvider.GetConfig()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "获取Nacos配置失败: " + err.Error()})
			return
		}
		
		// 检查源程序是否存在
		if !myini.HasSection("program:" + sourceProgramName) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "源程序 " + sourceProgramName + " 不存在"})
			return
		}
		
		// 检查新程序名称是否已存在
		if myini.HasSection("program:" + copyData.NewName) {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "程序 " + copyData.NewName + " 已存在"})
			return
		}
		
		// 将配置转换为字符串
		configStr := myini.String()
		
		// 查找源程序配置的开始位置
		startIndex := strings.Index(configStr, "[program:"+sourceProgramName+"]")
		if startIndex == -1 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "无法找到源程序配置"})
			return
		}
		
		// 查找下一个配置节的开始位置或文件结束
		endIndex := len(configStr)
		nextSectionIndex := strings.Index(configStr[startIndex+1:], "[")
		if nextSectionIndex != -1 {
			endIndex = startIndex + 1 + nextSectionIndex
		}
		
		// 提取源程序配置
		sourceConfig := configStr[startIndex:endIndex]
		
		// 创建新程序配置
		newConfig := strings.Replace(sourceConfig, "[program:"+sourceProgramName+"]", "[program:"+copyData.NewName+"]", 1)
		
		// 将新配置追加到文件
		newContent := configStr + "\n" + newConfig
		
		// 保存到Nacos
		err = nacosProvider.SaveConfig(newContent)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "保存Nacos配置失败: " + err.Error()})
			return
		}
	} else {
		// 使用本地文件配置
		
		// 读取现有的supervisor.conf文件
		configFile := sr.supervisor.GetConfig().GetConfigFileDir() + "/supervisor.conf"
		content, err := ioutil.ReadFile(configFile)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "读取配置文件失败: " + err.Error()})
			return
		}
		
		contentStr := string(content)
		
		// 检查源程序是否存在
		if !strings.Contains(contentStr, "[program:"+sourceProgramName+"]") {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "源程序 " + sourceProgramName + " 不存在"})
			return
		}
		
		// 检查新程序名称是否已存在
		if strings.Contains(contentStr, "[program:"+copyData.NewName+"]") {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "程序 " + copyData.NewName + " 已存在"})
			return
		}
		
		// 查找源程序配置的开始位置
		startIndex := strings.Index(contentStr, "[program:"+sourceProgramName+"]")
		if startIndex == -1 {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "无法找到源程序配置"})
			return
		}
		
		// 查找下一个配置节的开始位置或文件结束
		endIndex := len(contentStr)
		nextSectionIndex := strings.Index(contentStr[startIndex+1:], "[")
		if nextSectionIndex != -1 {
			endIndex = startIndex + 1 + nextSectionIndex
		}
		
		// 提取源程序配置
		sourceConfig := contentStr[startIndex:endIndex]
		
		// 创建新程序配置
		newConfig := strings.Replace(sourceConfig, "[program:"+sourceProgramName+"]", "[program:"+copyData.NewName+"]", 1)
		
		// 将新配置追加到文件
		newContent := contentStr + "\n" + newConfig
		
		// 写入文件
		err = ioutil.WriteFile(configFile, []byte(newContent), 0644)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "写入配置文件失败: " + err.Error()})
			return
		}
	}
	
	// 重新加载配置
	_, _, _, err := sr.supervisor.Reload(false)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "重新加载配置失败: " + err.Error()})
		return
	}
	
	// 返回成功
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "程序 " + sourceProgramName + " 已成功复制为 " + copyData.NewName,
		"source_program": sourceProgramName,
		"new_program": copyData.NewName,
	})
}

// ReadStdoutLog read the stdout of given program
func (sr *SupervisorRestful) ReadStdoutLog(w http.ResponseWriter, req *http.Request) {
}

// Shutdown the supervisor itself
func (sr *SupervisorRestful) Shutdown(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	reply := struct{ Ret bool }{false}
	sr.supervisor.Shutdown(nil, nil, &reply)
	w.Write([]byte("Shutdown..."))
}

// Reload the supervisor configuration file through rest interface
func (sr *SupervisorRestful) Reload(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()

	_, _, _, err := sr.supervisor.Reload(false)
	r := map[string]bool{"success": err == nil}
	json.NewEncoder(w).Encode(&r)
}

// GetNacosConfig 获取当前Nacos配置
func (sr *SupervisorRestful) GetNacosConfig(w http.ResponseWriter, req *http.Request) {
	// 从配置文件中读取Nacos配置
	configFile := sr.supervisor.GetConfig().GetConfigFileDir() + "/nacos.json"
	
	// 如果配置文件不存在，返回空配置
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"serverAddr": "",
			"namespace": "",
			"group": "DEFAULT_GROUP",
			"dataId": "",
			"username": "",
			"password": "",
		})
		return
	}
	
	// 读取配置文件
	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "读取Nacos配置文件失败: " + err.Error()})
		return
	}
	
	// 解析配置
	var nacosConfig config.NacosConfig
	if err := json.Unmarshal(content, &nacosConfig); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "解析Nacos配置文件失败: " + err.Error()})
		return
	}
	
	// 返回配置
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(nacosConfig)
}

// SaveNacosConfig 保存Nacos配置
func (sr *SupervisorRestful) SaveNacosConfig(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	
	// 解析请求体
	var nacosConfig config.NacosConfig
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&nacosConfig); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的请求数据: " + err.Error()})
		return
	}
	
	// 验证必填字段
	if nacosConfig.ServerAddr == "" || nacosConfig.DataId == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "服务器地址和配置ID不能为空"})
		return
	}
	
	// 将配置保存到文件
	configFile := sr.supervisor.GetConfig().GetConfigFileDir() + "/nacos.json"
	content, err := json.MarshalIndent(nacosConfig, "", "  ")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "序列化Nacos配置失败: " + err.Error()})
		return
	}
	
	// 写入文件
	err = ioutil.WriteFile(configFile, content, 0644)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "写入Nacos配置文件失败: " + err.Error()})
		return
	}
	
	// 返回成功
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Nacos配置已保存",
		"config": nacosConfig,
	})
}

// TestNacosConnection 测试Nacos连接
func (sr *SupervisorRestful) TestNacosConnection(w http.ResponseWriter, req *http.Request) {
	defer req.Body.Close()
	
	// 解析请求体
	var nacosConfig config.NacosConfig
	decoder := json.NewDecoder(req.Body)
	if err := decoder.Decode(&nacosConfig); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "无效的请求数据: " + err.Error()})
		return
	}
	
	// 验证必填字段
	if nacosConfig.ServerAddr == "" || nacosConfig.DataId == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "服务器地址和配置ID不能为空"})
		return
	}
	
	// 测试连接
	provider, err := config.NewNacosConfigProvider(nacosConfig)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "连接Nacos失败: " + err.Error()})
		return
	}
	
	// 尝试获取配置
	_, err = provider.GetConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "获取Nacos配置失败: " + err.Error()})
		return
	}
	
	// 返回成功
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Nacos连接测试成功",
	})
}
