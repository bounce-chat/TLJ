package tlj

import (
	"net"
	"sync"
	"reflect"
	"errors"
	"encoding/json"
)

type Client struct {
	Socket		net.Conn
	TypeStore	*TypeStore
	Requests	map[uint16]map[uint16][]func(interface{})
	NextID		uint16
	Writing		*sync.Mutex
	Inserting	*sync.Mutex
	Dead		chan error
}

type Request struct {
	RequestID	uint16
	Type		uint16
	Data		string
	Client		*Client
}

func NewClient(socket net.Conn, type_store *TypeStore) Client {
	client := Client {
		Socket:		socket,
		TypeStore:	type_store,
		Requests:	make(map[uint16]map[uint16][]func(interface{})),
		NextID:		1,
		Writing:	&sync.Mutex{},
		Inserting:	&sync.Mutex{},
		Dead:		make(chan error, 1),
	}
	go client.process()
	return client
}

func (client *Client) process() {
	for {
		capsule, err := nextStruct(client.Socket, client.TypeStore)
		if err != nil {
			client.Dead <- err
			break
		}
		if reflect.TypeOf(capsule) != reflect.TypeOf(Capsule{}) { continue }
		capsule_value := reflect.Indirect(reflect.ValueOf(capsule))
		capsule_request_id := uint16(capsule_value.FieldByName("RequestID").Uint())
		capsule_type_code := uint16(capsule_value.FieldByName("Type").Uint())
		capsule_data := capsule_value.FieldByName("Data").String()
		recieved_struct := client.TypeStore.BuildType(capsule_type_code, []byte(capsule_data))
		if recieved_struct == nil { continue }
		if client.Requests[capsule_request_id][capsule_type_code] == nil { continue }
		for _, function := range(client.Requests[capsule_request_id][capsule_type_code]) {
			go function(recieved_struct)
		}
	}
}

func (client *Client) getRequestID() uint16 {
	// cycle over old requests when id reaches max?
	// generate randomly until full?
	id := client.NextID
	client.NextID = id + 1
	return id
}

func (client *Client) Message(instance interface{}) error {
	message, err := format(instance, client.TypeStore)
	if err != nil { return err }
	client.Writing.Lock()
	_ , err = client.Socket.Write(message)
	client.Writing.Unlock()
	return err
}

func (client *Client) Request(instance interface{}) (*Request, error) {
	instance_data, err := json.Marshal(instance)
	if err != nil { return nil, err }
	instance_type, present := client.TypeStore.LookupCode(reflect.TypeOf(instance))
	if !present { return nil, errors.New("cannot request type not in type stores") }
	request := &Request {
		RequestID:	client.getRequestID(),
		Type:		instance_type,
		Data:		string(instance_data),
		Client:		client,
	}
	capsule := Capsule {
		RequestID:	request.RequestID,
		Type:		request.Type,
		Data:		request.Data,
	}
	client.Requests[request.RequestID] = make(map[uint16][]func(interface{}))
	err = client.Message(capsule)
	return request, err
}

func (request *Request) OnResponse(struct_type reflect.Type, function func(interface{})) {
	type_id, present := request.Client.TypeStore.LookupCode(struct_type)
	if !present { return }
	request.Client.Inserting.Lock()
	request.Client.Requests[request.RequestID][type_id] = append(request.Client.Requests[request.RequestID][type_id], function)
	request.Client.Inserting.Unlock()
}
