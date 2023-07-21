package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
  "crypto/tls"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Port              string `yaml:"port"`
	SSLEnabled        bool   `yaml:"sslEnabled"`
	SSLKeyFileName    string `yaml:"sslKeyFileName"`
	SSLCertFileName   string `yaml:"sslCertFileName"`
	GitlabURL         string `yaml:"gitlabURL"`
	GitlabAPIPrefix   string `yaml:"gitlabAPIPrefix"`
	GitlabAccessToken string `yaml:"gitlabAccessToken"`
	GitlabProjectID   string `yaml:"gitlabProjectID"`
}

type Alert struct {
	Receiver          string        `json:"receiver"`
	Status            string        `json:"status"`
	Info              []ArrayAlerts `json:"alerts"`
	CommonAnnotations Annotations   `json:"commonAnnotations"`
	CommonLabels      Labels        `json:commonLabels`
}

type ArrayAlerts struct {
	StartTime    string `json:"startsAt"`
	EndTime      string `json:"endsAt"`
	GeneratorURL string `json:"generatorURL"`
}

type Annotations struct {
	Description string `json:"description"`
	Summary     string `json:"summary"`
}

type Labels struct {
	AlertName string `json:"alertname"`
	Namespace string `json:"namespace"`
	Node      string `json:"node"`
	Service   string `json:"service"`
	Severity  string `json:"severity"`
}

func main() {

	config, err := parseYaml("/app/config/config.yaml")
	if err != nil {
		log.Fatal(err)
		return
	}
	log.Println("Start Webhook for alerting")
	http.HandleFunc("/", sendAlert)
	if config.SSLEnabled {
		log.Fatal(http.ListenAndServeTLS(config.Port, config.SSLCertFileName, config.SSLKeyFileName, nil))
	}
	log.Fatal(http.ListenAndServe(config.Port, nil))
}

func parseYaml(filename string) (Config, error) {
	yamlConfig, err := ioutil.ReadFile(filename)
	config := Config{}
	if err != nil {
		return config, err
	}
	errYaml := yaml.Unmarshal(yamlConfig, &config)
	if errYaml != nil {
		return config, err
	}
	return config, nil
}

func sendAlert(w http.ResponseWriter, r *http.Request) {
	config, err := parseYaml("/app/config/config.yaml")
	if err != nil {
		log.Fatal(err)
		return
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	log.Printf("Receiving: %s\n", string(b))
	alert, errJson := parseJson(b)
	if errJson != nil {
		writeError(w, errJson)
		return
	}
	data := createMessage(alert)
	body, errEnc := json.Marshal(data)
	if errEnc != nil {
		writeError(w, errEnc)
		return
	}
	URL := config.GitlabURL + config.GitlabAPIPrefix + config.GitlabProjectID + "/issues"
	req, errReq := http.NewRequest("POST", URL, bytes.NewBuffer(body))
	if errReq != nil {
		writeError(w, errReq)
		return
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("PRIVATE-TOKEN", config.GitlabAccessToken)
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	client := &http.Client{Transport: tr}
	resp, errResp := client.Do(req)
	if errResp != nil {
		writeError(w, errResp)
		return
	}
	defer resp.Body.Close()
	status, errStat := ioutil.ReadAll(resp.Body)
	if errStat != nil {
		writeError(w, errStat)
		return
	}
	log.Printf("Response from git:%s \n", string(status))
}

func parseJson(data []byte) (Alert, error) {
	alert := Alert{}
	err := json.Unmarshal(data, &alert)
	if err != nil {
		return alert, err
	}
	return alert, nil
}

func createMessage(alert Alert) map[string]string {
	title := fmt.Sprintf("ALERTMANAGER -> Namespace:%s Node:%s",
		alert.CommonLabels.Namespace, alert.CommonLabels.Node)
	description := fmt.Sprintf("Service: %s\n\n AlertName: %s\n\n Receiver: %s\n\n Status: %s\n\n StartTime: %s\n\n EndTime: %s\n\n",
		alert.CommonLabels.Service,
		alert.CommonLabels.AlertName,
		alert.Receiver, alert.Status,
		alert.Info[0].StartTime, alert.Info[0].EndTime)
	description += fmt.Sprintf("generatorURL: %s\n\n Description: %s\n\n Summary: %s\n\n",
		alert.Info[0].GeneratorURL,
		alert.CommonAnnotations.Description,
		alert.CommonAnnotations.Summary)
	body := map[string]string{"title": title, "description": description}
	return body
}

func writeError(w http.ResponseWriter, err error) {
	err = fmt.Errorf("Error: %v", err)
	w.WriteHeader(http.StatusInternalServerError) // 500
	fmt.Fprintln(w, err)
	log.Println(err)
}
