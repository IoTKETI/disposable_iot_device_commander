// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2018 Canonical Ltd
// Copyright (C) 2018-2019 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

// This package provides a simple example implementation of
// ProtocolDriver interface.
//
package driver

import (
	"fmt"
	"net"
	"strconv"
	"time"
	"io"

	flags "github.com/jessevdk/go-flags"
	dsModels "github.com/edgexfoundry/device-sdk-go/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/clients/logger"
	contract "github.com/edgexfoundry/go-mod-core-contracts/models"
)

type Options struct {
	UseRegistry   string `short:"r" long:"registry" description:"Indicates the service should use the registry and provide the registry url." optional:"true" optional-value:"LOAD_FROM_FILE"`
	ConfProfile   string `short:"p" long:"profile" description:"Specify a profile other than default."`
	ConfDir       string `short:"c" long:"confdir" description:"Specify an alternate configuration directory."`
	OverwriteConf bool   `short:"o" long:"overwrite" description:"Overwrite configuration in the registry."`
}

type SimpleDriver struct {
	lc           logger.LoggingClient
	asyncCh      chan<- *dsModels.AsyncValues
	switchButton bool
	xRotation    int32
	yRotation    int32
	zRotation    int32
}

var (
	conn net.Conn
	command_list map[string][2]string
	dri_in_MS int
	current_running_task map[string][]string
	IPE_addr = "localhost:8700"
)

func parse(str *string, start_index int, prefix string, field map[string]string) int {
        field_name := prefix
        cur_str := ""
        index := 0
        for start_index < len(*str) {
                ch := (*str)[start_index]
                start_index += 1

                switch string(ch) {
                case "{", "[":
                        if field_name == prefix {
                                field_name += "/"+strconv.Itoa(index)
                        }

                        start_index = parse(str, start_index, field_name, field)
                        index += 1
                        cur_str = ""
                        field_name = prefix
                case "}", ",", "]":
                        if cur_str != "" {
                                if (field_name == prefix) {
                                        field_name += "/"+strconv.Itoa(index)
                                }
                                _, is_exist := field[field_name]
                                if is_exist == false {
                                        field[field_name] = cur_str
                                }
                                field_name = prefix
                                cur_str = ""
                                index += 1
                        }

                        if string(ch) == "}" || string(ch) == "]" {
                                return start_index
                        }
                case "=":
                        field_name += "/"+cur_str
                        cur_str = ""
                case ";":
                        if (field_name == prefix) {
                                field_name += "/"+strconv.Itoa(index)
                        }
                        _, is_exist := field[field_name]
                        if is_exist == false {
                                field[field_name] = cur_str
                        }
                        field_name = prefix
                        cur_str = ""
                        index += 1
                default:
                        cur_str = cur_str + string(ch)
                }
        }

        for key, value := range field {
                fmt.Println(key+","+value)
        }
        return -1
}

func ReconnectToIPE(conn net.Conn){
	fmt.Println("IPE disconnected!!")
	for {
		fmt.Println("Try to connect to IPE!!")
		conn, _ = net.Dial("tcp",IPE_addr)
		if conn != nil {
			fmt.Println("Reconnect success!!")
			break
		}
		time.Sleep(10 * time.Second)
	}
}

func IPEMessageHandler(){
	recv_buf := make([]byte, 4096)

	if conn == nil {
		ReconnectToIPE(conn)
	}

	for {
		n, err := conn.Read(recv_buf)
		if err != nil {
			fmt.Println(err)
			conn.Close()
			conn = nil
			if err == io.EOF {
				ReconnectToIPE(conn)
				continue
			} else {
				return
			}
		}
		if n > 0 {
			data := string(recv_buf[:n])
			field := make(map[string]string)
			parse(&data, 0, "", field)
			if if_value, is_exist := field["/0/if"]; is_exist == true && if_value == "0" {
				dri_in_MS := field["/0/dri"]
				global_dri := field["/1/global_dri"]
				cmd_if := command_list[dri_in_MS][0]
				cmd_param := command_list[dri_in_MS][1]
				conn.Write([]byte("{if="+cmd_if+";dri="+global_dri+"}"+cmd_param))
				delete(command_list, dri_in_MS)
			}
		}
	}
}

// Initialize performs protocol-specific initialization for the device
// service.
func (s *SimpleDriver) Initialize(lc logger.LoggingClient, asyncCh chan<- *dsModels.AsyncValues) error {
	command_list = make(map[string][2]string)
	current_running_task = make(map[string][]string)
	var opts Options
	flags.Parse(&opts)
	if opts.ConfProfile == "docker" {
		IPE_addr = "dockerhost:8700"
	}
	fmt.Println(IPE_addr)
	conn, _ = net.Dial("tcp",IPE_addr)

	go IPEMessageHandler()

	s.lc = lc
	s.asyncCh = asyncCh
	return nil
}

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (s *SimpleDriver) HandleReadCommands(deviceName string, protocols map[string]contract.ProtocolProperties, reqs []dsModels.CommandRequest) (res []*dsModels.CommandValue, err error) {
	s.lc.Debug(fmt.Sprintf("SimpleDriver.HandleReadCommands: protocols: %v resource: %v attributes: %v", protocols, reqs[0].DeviceResourceName, reqs[0].Attributes))

	if reqs[0].DeviceResourceName == "taskToRun" {
		now := time.Now().UnixNano()
		res = make([]*dsModels.CommandValue, 1)
		task_list := "["
		for _, task := range current_running_task[deviceName] {
			task_list += task+";"
		}
		task_list += "]"
		cv := dsModels.NewStringValue(reqs[0].DeviceResourceName, now, task_list)
		res[0] = cv
	}

	/*if len(reqs) == 1 {
		res = make([]*dsModels.CommandValue, 1)
		now := time.Now().UnixNano()
		if reqs[0].DeviceResourceName == "SwitchButton" {
			cv, _ := dsModels.NewBoolValue(reqs[0].DeviceResourceName, now, s.switchButton)
			res[0] = cv
		} else if reqs[0].DeviceResourceName == "Image" {
			// Show a binary/image representation of the switch's on/off value
			buf := new(bytes.Buffer)
			if s.switchButton == true {
				err = getImageBytes("./res/on.png", buf)
			} else {
				err = getImageBytes("./res/off.jpg", buf)
			}
			cvb, _ := dsModels.NewBinaryValue(reqs[0].DeviceResourceName, now, buf.Bytes())
			res[0] = cvb
		} else if reqs[0].DeviceResourceName == "sjlee" {
			cv, _ := dsModels.NewInt32Value(reqs[0].DeviceResourceName, now, int32(rand.Intn(100)))
			res[0] = cv
		}
	} else if len(reqs) == 3 {
		res = make([]*dsModels.CommandValue, 3)
		for i, r := range reqs {
			var cv *dsModels.CommandValue
			now := time.Now().UnixNano()
			switch r.DeviceResourceName {
			case "Xrotation":
				cv, _ = dsModels.NewInt32Value(r.DeviceResourceName, now, s.xRotation)
			case "Yrotation":
				cv, _ = dsModels.NewInt32Value(r.DeviceResourceName, now, s.yRotation)
			case "Zrotation":
				cv, _ = dsModels.NewInt32Value(r.DeviceResourceName, now, s.zRotation)
			}
			res[i] = cv
		}
	}*/

	return
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource.
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (s *SimpleDriver) HandleWriteCommands(deviceName string, protocols map[string]contract.ProtocolProperties, reqs []dsModels.CommandRequest,
	params []*dsModels.CommandValue) error {
	s.lc.Debug(fmt.Sprintf("SimpleDriver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v", protocols, reqs[0].DeviceResourceName, params))
	cmd_if := ""
	cmd_param := ""
	if len(reqs) == 1 {
		switch reqs[0].DeviceResourceName {
		case "task":
			task, _ := params[0].StringValue()
			cmd_if = "3"
			cmd_param = "{tis=["+task+"]}"
		case "taskToRun":
			task_to_run, _ := params[0].StringValue()
			cmd_if = "5"
			cmd_param = "{tis=["+task_to_run+"]}"
			current_running_task[deviceName] = append(current_running_task[deviceName], task_to_run)
		case "taskToRead":
			task_to_read, _ := params[0].StringValue()
			cmd_if = "7"
			op := ""
			switch task_to_read {
			case "101","102","103":
				op = "atemp"
			case "104":
				op = "temp"
			case "201","202","203":
				op = "ahum"
			case "204":
				op = "hum"
			}
			cmd_param ="{tis=["+task_to_read+"];op=["+op+"]}"
		case "taskToStop":
			task_to_stop, _ := params[0].StringValue()
			cmd_if = "9"
			cmd_param = "{tis=["+task_to_stop+"]}"
		}
	} else if len(reqs) == 6 {
		task_to_change, _ := params[0].StringValue()
		cmd_if = "4"
		cmd_param = "{tis=[{ti="+task_to_change+";fp={"
		for index, param := range params[1:] {
			fp_value, _ := param.StringValue()
			fmt.Println(reqs[index+1].DeviceResourceName+":"+fp_value)
			if fp_value != "" {
				cmd_param += reqs[index+1].DeviceResourceName+"="+fp_value+";"
			}
		}
		cmd_param += "};}]}"
	}

	dri := strconv.Itoa(dri_in_MS)
	command_list[dri] = [2]string{cmd_if, cmd_param}
	conn.Write([]byte("{if=0;dri="+dri+"}{di="+deviceName+"}"))

	return nil
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (s *SimpleDriver) Stop(force bool) error {
	// Then Logging Client might not be initialized
	if s.lc != nil {
		s.lc.Debug(fmt.Sprintf("SimpleDriver.Stop called: force=%v", force))
	}
	return nil
}

// AddDevice is a callback function that is invoked
// when a new Device associated with this Device Service is added
func (s *SimpleDriver) AddDevice(deviceName string, protocols map[string]contract.ProtocolProperties, adminState contract.AdminState) error {
	s.lc.Debug(fmt.Sprintf("a new Device is added: %s", deviceName))
	return nil
}

// UpdateDevice is a callback function that is invoked
// when a Device associated with this Device Service is updated
func (s *SimpleDriver) UpdateDevice(deviceName string, protocols map[string]contract.ProtocolProperties, adminState contract.AdminState) error {
	s.lc.Debug(fmt.Sprintf("Device %s is updated", deviceName))
	return nil
}

// RemoveDevice is a callback function that is invoked
// when a Device associated with this Device Service is removed
func (s *SimpleDriver) RemoveDevice(deviceName string, protocols map[string]contract.ProtocolProperties) error {
	s.lc.Debug(fmt.Sprintf("Device %s is removed", deviceName))
	return nil
}
