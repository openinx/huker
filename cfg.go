package huker

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// The abstract functions for configuration file.
type ConfigFile interface {
	mergeWith(c ConfigFile) ConfigFile
	toString() string
	toKeyValue() map[string]string
	getConfigName() string
}

// Configuration file with .ini, .properties
type INIConfigFile struct {
	cfgName   string
	keyValues []string
}

// Create a new .ini config files.
func NewINIConfigFile(cfgName string, keyValues []string) INIConfigFile {
	return INIConfigFile{
		cfgName:   cfgName,
		keyValues: keyValues,
	}
}

func (c INIConfigFile) mergeWith(other ConfigFile) ConfigFile {
	cMap := c.toKeyValue()
	oMap := other.toKeyValue()
	// If key exist in both cMap and oMap, then use value of cMap.
	for key, val := range cMap {
		oMap[key] = val
	}
	// convert oMap to []string
	keyValues := []string{}
	for key, val := range oMap {
		keyValues = append(keyValues, fmt.Sprintf("%s=%s", key, val))
	}
	c.keyValues = keyValues
	return c
}

func (c INIConfigFile) toString() string {
	return strings.Join(c.keyValues, "\n")
}

func (c INIConfigFile) toKeyValue() map[string]string {
	ret := make(map[string]string)
	for i := range c.keyValues {
		parts := strings.SplitN(c.keyValues[i], "=", 2)
		if len(parts) != 2 {
			panic(fmt.Sprintf("Invalid key value pair, key or value not found. %s", c.keyValues[i]))
		}
		ret[parts[0]] = parts[1]
	}
	return ret
}

func (c INIConfigFile) getConfigName() string {
	return c.cfgName
}

// Configuration file with xml format
type XMLConfigFile struct {
	cfgName   string
	keyValues []string
}

// Create a new xml config files.
func NewXMLConfigFile(cfgName string, keyValues []string) XMLConfigFile {
	return XMLConfigFile{
		cfgName:   cfgName,
		keyValues: keyValues,
	}
}

func (c XMLConfigFile) mergeWith(other ConfigFile) ConfigFile {
	cMap := c.toKeyValue()
	oMap := other.toKeyValue()
	// If key exist in both cMap and oMap, the use value of cMap.
	for key, val := range cMap {
		oMap[key] = val
	}
	// convert oMap to []string
	keyValues := []string{}
	for key, val := range oMap {
		keyValues = append(keyValues, fmt.Sprintf("%s=%s", key, val))
	}
	c.keyValues = keyValues
	return c
}

func (c XMLConfigFile) toString() string {
	var buf []string
	buf = append(buf, "<configuration>")

	kvMap := c.toKeyValue()
	for key := range kvMap {
		buf = append(buf, "  <property>")
		buf = append(buf, fmt.Sprintf("    <name>%s</name>", key))
		buf = append(buf, fmt.Sprintf("    <value>%s</value>", kvMap[key]))
		buf = append(buf, "  </property>")
	}
	buf = append(buf, "</configuration>")

	return strings.Join(buf, "\n")
}

func (c XMLConfigFile) toKeyValue() map[string]string {
	ret := make(map[string]string)
	for i := range c.keyValues {
		parts := strings.SplitN(c.keyValues[i], "=", 2)
		if len(parts) != 2 {
			panic(fmt.Sprintf("Invalid key value pair, key or value not found. %s", c.keyValues[i]))
		}
		ret[parts[0]] = parts[1]
	}
	return ret
}

func (c XMLConfigFile) getConfigName() string {
	return c.cfgName
}

// Configuration file with plain format, which means can be any format.
type PlainConfigFile struct {
	cfgName string
	lines   []string
}

// New a plain configuration file.
func NewPlainConfigFile(cfgName string, lines []string) PlainConfigFile {
	return PlainConfigFile{
		cfgName: cfgName,
		lines:   lines,
	}
}

func (c PlainConfigFile) mergeWith(other ConfigFile) ConfigFile {
	for _, line := range other.toKeyValue() {
		c.lines = append(c.lines, line)
	}
	return c
}

func (c PlainConfigFile) toString() string {
	var buf []string
	for _, line := range c.lines {
		buf = append(buf, line)
	}
	return strings.Join(buf, "\n")
}

func (c PlainConfigFile) toKeyValue() map[string]string {
	ret := make(map[string]string)
	for i, line := range c.lines {
		ret[strconv.Itoa(i)] = line
	}
	return ret
}

func (c PlainConfigFile) getConfigName() string {
	return c.cfgName
}

// Initialize the concrete configuration file by the suffix of cfgName.
func ParseConfigFile(cfgName string, keyValues []string) (ConfigFile, error) {
	fname := filepath.Base(cfgName)
	if strings.HasSuffix(fname, ".cfg") || strings.HasSuffix(fname, ".properties") {
		return NewINIConfigFile(cfgName, keyValues), nil
	} else if strings.HasSuffix(fname, ".xml") {
		return NewXMLConfigFile(cfgName, keyValues), nil
	} else if !strings.Contains(fname, ".") || strings.HasSuffix(fname, ".txt") {
		return NewPlainConfigFile(cfgName, keyValues), nil
	}
	return nil, fmt.Errorf("Unsupported configuration file format. %s", cfgName)
}
