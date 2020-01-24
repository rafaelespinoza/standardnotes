package config

type Config struct {
	DB      string `json:"db"`
	Debug   bool   `json:"debug"`
	Host    string `json:"host"`
	NoReg   bool   `json:"noreg"`
	Port    int    `json:"port"`
	Socket  string `json:"socket"`
	UseCORS bool   `json:"cors"`
}

var Conf = Config{
	DB:      "sf.db",
	Debug:   false,
	NoReg:   false,
	Port:    8888,
	UseCORS: false,
}

var Metadata = struct {
	Version      string
	LoadedConfig string
}{}
