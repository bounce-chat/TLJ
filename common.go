package tlj

import (
	"net"
	"reflect"
	"errors"
	"encoding/json"
	"encoding/binary"
)

type Capsule struct {
	RequestID	uint16
	Type		uint16
	Data		string
}

type Builder func([]byte) interface{}

type TypeStore struct {
	Types			map[uint16]Builder
	TypeCodes		map[reflect.Type]uint16
	NextID			uint16
}

func NewTypeStore() TypeStore {
	type_store := TypeStore {
		Types:		make(map[uint16]Builder),
		TypeCodes:	make(map[reflect.Type]uint16),
		NextID:		1,
	}
	
	capsule_builder := func(data []byte) interface{} {
		capsule := &Capsule{}
		err := json.Unmarshal(data, &capsule)
		if err != nil { return nil }
		return capsule
	}
	type_store.Types[0] = capsule_builder
	type_store.TypeCodes[reflect.TypeOf(Capsule{})] = 0
	//type_store.TypeCodes[reflect.TypeOf(&Capsule{})] = 0
	
	return type_store
}

func (store *TypeStore) AddType(inst_type reflect.Type, ptr_type reflect.Type, builder Builder) {
	type_id := store.NextID
	store.NextID = store.NextID + 1
	store.Types[type_id] = builder
	store.TypeCodes[inst_type] = type_id
	store.TypeCodes[ptr_type] = type_id
}

func (store *TypeStore) LookupCode(struct_type reflect.Type) (uint16, bool) {
	val, present := store.TypeCodes[struct_type]
	return val, present
}

func (store *TypeStore) BuildType(struct_code uint16, data []byte) interface{} {
	function, present := store.Types[struct_code]
	if !present { return nil }
	return function(data)
}

func format(instance interface{}, type_store *TypeStore) ([]byte, error) {
	bytes, err := json.Marshal(instance)
	if err != nil { return nil, err }
	
	type_bytes := make([]byte, 2)
	struct_type, present := type_store.LookupCode(reflect.TypeOf(instance))
	if !present { return type_bytes, errors.New("struct type missing from TypeStore") }
	binary.LittleEndian.PutUint16(type_bytes, struct_type)
	
	length := len(bytes)
	length_bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(length_bytes, uint32(length))
	
	bytes = append(type_bytes, append(length_bytes, bytes...)...)

	return bytes, err
}

func formatCapsule(instance interface{}, type_store *TypeStore, request_id uint16) ([]byte, error) {
	bytes, err := json.Marshal(instance)
	if err != nil { return bytes, err }

	struct_type, present := type_store.LookupCode(reflect.TypeOf(instance))
	if !present { return bytes, errors.New("struct type missing from TypeStore") }
	
	capsule := Capsule {
		RequestID:	request_id,
		Type:		struct_type,
		Data:		string(bytes),
	}

	return format(capsule, type_store)
}

func nextStruct(socket net.Conn, type_store *TypeStore) (interface{}, error) {
	header := make([]byte, 6)
	n, err := socket.Read(header)
	if err != nil { return nil, err }
	if n != 6 { return nil, nil }
	
	type_bytes := header[:2]
	size_bytes := header[2:]
	
	type_int := binary.LittleEndian.Uint16(type_bytes)
	size_int := binary.LittleEndian.Uint32(size_bytes)

	struct_data := make([]byte, size_int)
	_, err = socket.Read(struct_data)
	if err != nil { return nil, err }
	
	recieved_struct := type_store.BuildType(type_int, struct_data)
	
	return recieved_struct, nil	
}
