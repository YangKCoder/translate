package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

const translateUrl = "https://api.fanyi.baidu.com/api/trans/vip/translate?appid=%s&salt=%s&sign=%s&from=auto&to=%s&q=%s"

var (
	appId     = os.Getenv("TRANSLATE_APPID")
	secret    = os.Getenv("TRANSLATE_SECRET")
	salt      = "baidu"
	dir, _    = Home()
	localFile = dir + "/translate.json"
	locale    = "zh"
	c         = flag.Bool("c", false, "-c 英译汉")
)

func init() {
	exists, _ := PathExists(localFile)
	if !exists {
		if _, err := os.Create(localFile); err != nil {
			fmt.Fprintf(os.Stderr, "create local cache file fail : %v", err)
		}
	}
}

func main() {
	flag.Parse()
	if len(flag.Args()) == 0 {
		fmt.Fprintf(os.Stdout, "请传入要翻译的文本\n")
		return
	}
	if *c {
		locale = "en"
	}
	content := ""
	for _, arg := range flag.Args() {
		content += arg
	}
	result := Translate(content)
	fmt.Fprintf(os.Stdout, "结果: \nsrc: %s \t dst: %s \n", content, result)
}

func Translate(content string) string {
	var r Result
	//从本地缓存中读取
	re, b := ReadFormLocalFile(content)
	if b {
		r = re
	} else {
		resp, err := http.Get(fmt.Sprintf(translateUrl, appId, salt, generateSign(content), locale, content))

		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v", err)
		}
		defer resp.Body.Close()
		result, _ := ioutil.ReadAll(resp.Body)

		if err := json.Unmarshal(result, &r); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v", err)
		}

		WriteFormLocalFile(r)
	}
	var dstArr []string
	for _, dst := range r.TransResult {
		dstArr = append(dstArr, dst.Dst)
	}
	return strings.Join(dstArr, ",")
}

func generateSign(content string) string {
	w := md5.New()
	io.WriteString(w, appId+content+salt+secret)
	return fmt.Sprintf("%x", w.Sum(nil))
}

func ReadFormLocalFile(src string) (Result, bool) {
	b, _ := ioutil.ReadFile(localFile)
	var arr []Result
	if err := json.Unmarshal(b, &arr); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
	}
	for _, r := range arr {
		for _, t := range r.TransResult {
			if t.Src == src {
				return r, true
			}
		}
	}
	return Result{}, false
}

func WriteFormLocalFile(r Result) {
	b, _ := ioutil.ReadFile(localFile)
	var arr []Result
	json.Unmarshal(b, &arr)

	arr = append(arr, r)
	b, _ = json.Marshal(arr)
	ioutil.WriteFile(localFile, b, 0666)
}

func PathExists(path string) (bool, error) {

	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func Home() (string, error) {
	u, err := user.Current()
	if nil == err {
		return u.HomeDir, nil
	}

	// cross compile support

	if "windows" == runtime.GOOS {
		return homeWindows()
	}

	// Unix-like system, so just assume Unix
	return homeUnix()
}

func homeUnix() (string, error) {
	// First prefer the HOME environmental variable
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}
	// If that fails, try the shell
	var stdout bytes.Buffer
	cmd := exec.Command("sh", "-c", "eval echo ~$USER")
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}

	result := strings.TrimSpace(stdout.String())
	if result == "" {
		return "", errors.New("blank output when reading home directory")
	}
	return result, nil
}

func homeWindows() (string, error) {
	drive := os.Getenv("HOMEDRIVE")
	path := os.Getenv("HOMEPATH")
	home := drive + path
	if drive == "" || path == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		return "", errors.New("HOMEDRIVE, HOMEPATH, and USERPROFILE are blank")
	}

	return home, nil
}

type Result struct {
	From        string        `json:"from"`
	To          string        `json:"to"`
	TransResult []TransResult `json:"trans_result"`
}

type TransResult struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}
