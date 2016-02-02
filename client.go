package tlj

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"sync"
)

type Client struct {
	Socket    net.Conn
	TypeStore TypeStore
	Requests  map[uint16]map[uint16][]func(interface{})
	NextID    uint16
	Writing   *sync.Mutex
	Inserting *sync.Mutex
	Dead      chan error
}

func NewClient(socket net.Conn, type_store TypeStore) Client {
	client := Client{
		Socket:    socket,
		TypeStore: type_store,
		Requests:  make(map[uint16]map[uint16][]func(interface{})),
		NextID:    1,
		Writing:   &sync.Mutex{},
		Inserting: &sync.Mutex{},
		Dead:      make(chan error, 1),
	}
	go client.process()
	return client
}

func (client *Client) process() {
	context := TLJContext{
		Socket: client.Socket,
	}
	for {
		iface, err := client.TypeStore.NextStruct(client.Socket, context)
		if err != nil {
			client.Dead <- err
			break
		}
		if capsule, ok := iface.(*Capsule); ok {
			recieved_struct := client.TypeStore.BuildType(capsule.Type, []byte(capsule.Data), context)
			if recieved_struct == nil {
				continue
			}
			if client.Requests[capsule.RequestID][capsule.Type] == nil {
				continue
			}
			for _, function := range client.Requests[capsule.RequestID][capsule.Type] {
				go function(recieved_struct)
			}
		}
	}
}

func (client *Client) getRequestID() uint16 {
	id := client.NextID
	client.NextID = id + 1
	return id
}

func (client *Client) Message(instance interface{}) error {
	message, err := client.TypeStore.Format(instance)
	if err != nil {
		return err
	}
	client.Writing.Lock()
	_, err = client.Socket.Write(message)
	client.Writing.Unlock()
	return err
}

func (client *Client) Request(instance interface{}) (*Request, error) {
	instance_data, err := json.Marshal(instance)
	if err != nil {
		return nil, err
	}
	instance_type, present := client.TypeStore.LookupCode(reflect.TypeOf(instance))
	if !present {
		return nil, errors.New("cannot request type not in type stores")
	}
	request := &Request{
		RequestID: client.getRequestID(),
		Type:      instance_type,
		Data:      string(instance_data),
		Client:    client,
	}
	capsule := Capsule{
		RequestID: request.RequestID,
		Type:      request.Type,
		Data:      request.Data,
	}
	client.Inserting.Lock()
	client.Requests[request.RequestID] = make(map[uint16][]func(interface{}))
	client.Inserting.Unlock()
	err = client.Message(capsule)
	return request, err
}

type StreamWriter struct {
	Socket net.Conn
	TypeID uint16
}

func NewStreamWriter(conn net.Conn, type_store TypeStore, struct_type reflect.Type) (StreamWriter, error) {
	writer := StreamWriter{
		Socket: conn,
	}
	if conn == nil {
		return writer, errors.New("must provide conn")
	}
	type_id, present := type_store.LookupCode(struct_type)
	if !present {
		return writer, errors.New("no type ID in type store")
	}
	writer.TypeID = type_id
	return writer, nil
}

func (writer *StreamWriter) Write(obj interface{}) error {
	bytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	type_bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(type_bytes, writer.TypeID)

	length := len(bytes)
	length_bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(length_bytes, uint32(length))

	bytes = append(type_bytes, append(length_bytes, bytes...)...)
	writer.Socket.Write(bytes)
	return nil
}

type Request struct {
	RequestID uint16
	Type      uint16
	Data      string
	Client    *Client
}

func (request *Request) OnResponse(struct_type reflect.Type, function func(interface{})) {
	type_id, present := request.Client.TypeStore.LookupCode(struct_type)
	if !present {
		return
	}
	request.Client.Inserting.Lock()
	request.Client.Requests[request.RequestID][type_id] = append(request.Client.Requests[request.RequestID][type_id], function)
	request.Client.Inserting.Unlock()
}
