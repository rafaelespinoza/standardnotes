package main

import (
	"encoding/json"
	"io/ioutil"
)

type config struct {
	DB         string `json:"db"`
	Port       int    `json:"port"`
	Socket     string `json:"socket"`
	NoReg      bool   `json:"noreg"`
	Debug      bool   `json:"debug"`
	Foreground bool   `json:"foreground"`
	UseCORS    bool   `json:"cors"`
}

var _Config = config{
	DB:         "sf.db",
	Port:       8888,
	Debug:      false,
	NoReg:      false,
	Foreground: false,
	UseCORS:    false,
}

func initConf(path string, conf *config) (err error) {
	if data, ierr := ioutil.ReadFile(path); ierr != nil {
		err = ierr
		return
	} else if ierr = json.Unmarshal(data, conf); ierr != nil {
		err = ierr
		return
	}
	return
}
