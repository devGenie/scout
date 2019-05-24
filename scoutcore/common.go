package scoutcore

import (
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type configuration struct {
	name string
}

func ParseYml(filename string, config interface{}) error {
	source, err := ioutil.ReadFile(filename)

	if err != nil {
		return err
	}

	err = yaml.Unmarshal(source, config)
	if err != nil {
		return err
	}

	return nil
}
