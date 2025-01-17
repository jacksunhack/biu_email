package main

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Application struct {
		Name    string `yaml:"name"`
		Version string `yaml:"version"`
	} `yaml:"application"`
	Paths struct {
		Database          string `yaml:"database"`
		GenerateShortLink string `yaml:"generate_short_link"`
		GetFile           string `yaml:"get_file"`
		GetMessage        string `yaml:"get_message"`
		IndexHTML         string `yaml:"index_html"`
		MainGo            string `yaml:"main_go"`
		Redirect          string `yaml:"redirect"`
		SaveFile          string `yaml:"save_file"`
		SaveMessage       string `yaml:"save_message"`
	} `yaml:"paths"`
	Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	} `yaml:"server"`
	Security struct {
		EncryptionKeyLength int    `yaml:"encryption_key_length"`
		EncryptionAlgorithm string `yaml:"encryption_algorithm"`
	} `yaml:"security"`
	Frontend struct {
		Theme        string `yaml:"theme"`
		MatrixEffect bool   `yaml:"matrix_effect"`
		Styles       struct {
			BackgroundColor string `yaml:"background_color"`
			TextColor       string `yaml:"text_color"`
			FontFamily      string `yaml:"font_family"`
			MaxWidth        string `yaml:"max_width"`
			Margin          string `yaml:"margin"`
			Padding         string `yaml:"padding"`
			Border          string `yaml:"border"`
			BoxShadow       string `yaml:"box_shadow"`
			BorderRadius    string `yaml:"border_radius"`
		} `yaml:"styles"`
		Fonts struct {
			Primary   string `yaml:"primary"`
			Secondary string `yaml:"secondary"`
		} `yaml:"fonts"`
		AdSpace struct {
			Width   string `yaml:"width"`
			Padding string `yaml:"padding"`
		} `yaml:"ad_space"`
		Buttons struct {
			Default struct {
				BackgroundColor string `yaml:"background_color"`
				TextColor       string `yaml:"text_color"`
				Border          string `yaml:"border"`
				Padding         string `yaml:"padding"`
				BorderRadius    string `yaml:"border_radius"`
				LetterSpacing   string `yaml:"letter_spacing"`
				FontWeight      string `yaml:"font_weight"`
				BoxShadow       string `yaml:"box_shadow"`
			} `yaml:"default"`
			Hover struct {
				BackgroundColor string `yaml:"background_color"`
				TextColor       string `yaml:"text_color"`
				BoxShadow       string `yaml:"box_shadow"`
			} `yaml:"hover"`
			Active struct {
				Transform string `yaml:"transform"`
			} `yaml:"active"`
		} `yaml:"buttons"`
	} `yaml:"frontend"`
	APIEndpoints struct {
		SaveFile          string `yaml:"save_file"`
		GetFile           string `yaml:"get_file"`
		SaveMessage       string `yaml:"save_message"`
		GetMessage        string `yaml:"get_message"`
		GenerateShortLink string `yaml:"generate_short_link"`
	} `yaml:"api_endpoints"`
	Logging struct {
		Level    string `yaml:"level"`
		Format   string `yaml:"format"`
		Handlers struct {
			Console struct{} `yaml:"console"`
			File     struct {
				Path string `yaml:"path"`
			} `yaml:"file"`
		} `yaml:"handlers"`
	} `yaml:"logging"`
	Messages struct {
		EncryptionSuccess string `yaml:"encryption_success"`
		EncryptionError   string `yaml:"encryption_error"`
		UploadSuccess     string `yaml:"upload_success"`
		UploadError       string `yaml:"upload_error"`
		DownloadSuccess   string `yaml:"download_success"`
		DownloadError     string `yaml:"download_error"`
		InvalidParameters string `yaml:"invalid_parameters"`
	} `yaml:"messages"`
	UIText struct {
		EncryptAndTransmit string `yaml:"encrypt_and_transmit"`
		SwitchMode         string `yaml:"switch_mode"`
		EncryptedMessage   string `yaml:"encrypted_message"`
		EncryptedFile      string `yaml:"encrypted_file"`
		Loading            string `yaml:"loading"`
		Error              string `yaml:"error"`
		Success            string `yaml:"success"`
	} `yaml:"ui_text"`
	Database struct {
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Name     string `yaml:"name"`
	} `yaml:"database"`
}

func LoadConfig(configFile string) (*Config, error) {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}
	return &config, nil
}
