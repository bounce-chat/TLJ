package tlj

import (
	"net"
	"reflect"
	"encoding/json"
	"encoding/binary"
	"testing"
)

type Thingy struct {
	Name	string
	ID		int
}

func BuildThingy(data []byte) interface{} {
		thing := &Thingy{}
		err := json.Unmarshal(data, &thing)
		if err != nil { return nil }
		return thing
}

func TestTypeStoreIsCorrectType(t *testing.T) {
	type_store := NewTypeStore()
	if reflect.TypeOf(type_store) != reflect.TypeOf(TypeStore{}) {
		t.Errorf("return value of NewTypeStore() != tlj.TypeStore")
	} 
}

func TestTypeStoreHasCapsuleBuilder(t *testing.T) {
	type_store := NewTypeStore()
	cap := Capsule {
		RequestID:	1,
		Type:		1,
		Data:		"test",
	}
	cap_bytes, _ := json.Marshal(cap)
	iface := type_store.BuildType(0, cap_bytes)
	if restored, ok := iface.(*Capsule); ok {
		if restored.RequestID != cap.RequestID {
			t.Errorf("capsule builder did not restore RequestID")
		}
		if restored.Type != cap.Type {
			t.Errorf("capsule builder did not restore Type")
		}
		if restored.Data != cap.Data {
			t.Errorf("capsule builder did not restore Data")
		}
	} else {
		t.Errorf("could not assert *Capsule type on restored interface")
	}
}

func TestTypeStoreCanAddType(t *testing.T) {
	type_store := NewTypeStore()
	thingy_type := reflect.TypeOf(Thingy{})
	type_store.AddType(thingy_type, BuildThingy)
	if type_store.TypeCodes[thingy_type] != 1 {
		t.Errorf("call to AddType on new TypeStore did not create type_id of 1")
	}
}

func TestTypeStoreCanLookupCode(t *testing.T) {
	type_store := NewTypeStore()
	code, present := type_store.LookupCode(reflect.TypeOf(Capsule{}))
	if code != 0 || !present {
		t.Errorf("unable to lookup type_code for Capsule")
	}
}

func TestTypeStoreWontLookupBadCode(t *testing.T) {
	type_store := NewTypeStore()
	_, present := type_store.LookupCode(reflect.TypeOf(Thingy{}))
	if present {
		t.Errorf("nonexistent type returns a code")
	}
}

func TestTypeStoreCanBuildType(t *testing.T) {
	type_store := NewTypeStore()
	thingy_type := reflect.TypeOf(Thingy{})
	type_store.AddType(thingy_type, BuildThingy)
	if type_store.TypeCodes[thingy_type] != 1 {
		t.Errorf("call to AddType on new TypeStore did not create type_id of 1")
	}
	thingy := Thingy {
		Name:	"test",
		ID:		1,
	}
	marshalled, err := json.Marshal(thingy)
	if err != nil {
		t.Errorf("marshalling thingy returned an error")
	}
	iface := type_store.BuildType(1, marshalled)
	if restored, ok := iface.(*Thingy); ok {
		if restored.Name != thingy.Name {
			t.Errorf("string not presevered when building from marshalled struct")
		}
		if restored.ID != thingy.ID {
			t.Errorf("int not presevered when building from marshalled struct")
		}
	} else {
		t.Errorf("could not assert *Thingy type on restored interface")
	}
}

func TestTypeStoreWontBuildBadType(t *testing.T) {
	type_store := NewTypeStore()
	iface := type_store.BuildType(1, make([]byte, 0))
	if iface != nil {
		t.Errorf("type_store built something with a nonexistent id")
	}
}

func TestTypeStoreWontBuildUnformattedData(t *testing.T) {
	type_store := NewTypeStore()
	iface := type_store.BuildType(0, []byte("notjson"))
	if iface != nil {
		t.Errorf("type_store built something when bad data was supplied")
	}
}

func TestFormat(t *testing.T) {
	type_store := NewTypeStore()
	type_store.AddType(reflect.TypeOf(Thingy{}), BuildThingy)
	thing := Thingy {
		Name:	"test",
		ID:		1,
	}
	bytes, err := format(thing, &type_store)
	if err != nil {
		t.Errorf("error formatting valid struct: %s", err)
	}
	type_bytes := bytes[:2]
	size_bytes := bytes[2:6]
	json_data := bytes[6:]
	type_int := binary.LittleEndian.Uint16(type_bytes)
	size_int := binary.LittleEndian.Uint32(size_bytes)
	if type_int != 1 {
		t.Errorf("format didn't use the correct type ID")
	}
	if int(size_int) != len(json_data) {
		t.Errorf("format didn't set the correct length")
	}
	restored_thing := &Thingy{}
	err = json.Unmarshal(json_data, &restored_thing)
	if err != nil {
		t.Errorf("error unmarahalling format data: %s", err)
	}
}

func TestCantFormatUnknownType(t *testing.T) {
	type_store := NewTypeStore()
	thing := Thingy {
		Name:	"test",
		ID:		1,
	}
	_, err := format(thing, &type_store)
	if err == nil {
		t.Errorf("format didn't return error when unknown type was passed in")
	}
}

func TestFormatCapsule(t *testing.T) {
	type_store := NewTypeStore()
	type_store.AddType(reflect.TypeOf(Thingy{}), BuildThingy)
	thing := Thingy {
		Name:	"test",
		ID:		1,
	}
	bytes, err := formatCapsule(thing, &type_store, 1)
	if err != nil {
		t.Errorf("error formatting capsule with a thingy: %s", err)
	}
	type_bytes := bytes[:2]	
	size_bytes := bytes[2:6]
	capsule_data := bytes[6:]
	type_int := binary.LittleEndian.Uint16(type_bytes)
	size_int := binary.LittleEndian.Uint32(size_bytes)
	if type_int != 0 {
		t.Errorf("formatCapsule didn't use the correct type ID")
	}
	if int(size_int) != len(capsule_data) {
		t.Errorf("formatCapsule didn't set the correct length")
	}
	restored_capsule := &Capsule{}
	err = json.Unmarshal(capsule_data, &restored_capsule)
	if err != nil {
		t.Errorf("could not unmarshal formatted capsule")
	}
	if restored_capsule.RequestID != 1 {
		t.Errorf("formatCapsule didn't set the correct RequestID")
	}
	if restored_capsule.Type != 1 {
		t.Errorf("formatCapsule didn't set the correct Type")
	}
	restored_thing := &Thingy{}
	err = json.Unmarshal([]byte(restored_capsule.Data), &restored_thing)
	if err != nil {
		t.Errorf("could not unmarshal capsule data")
	}
	if restored_thing.Name != "test" {
		t.Errorf("capsuled thingy didn't come back with same name")
	}
	if restored_thing.ID != 1 {
		t.Errorf("capsuled thingy didn't come back with same ID")
	}
}

func TestCantFormatCapsuleWithUnknownType(t *testing.T) {
	type_store := NewTypeStore()
	thing := Thingy {
		Name:	"test",
		ID:		1,
	}
	_, err := formatCapsule(thing, &type_store, 1)
	if err == nil {
		t.Errorf("formatCapsule didn't error when it recieved a bad type")
	}
}

func TestNextStruct(t *testing.T) {
	type_store := NewTypeStore()
	type_store.AddType(reflect.TypeOf(Thingy{}), BuildThingy)
	sockets := make(chan net.Conn, 1)
	server, err := net.Listen("tcp", "localhost:5000")
	if err != nil {
		t.Errorf("could not start test server on localhost:5000")
	}
	defer server.Close()
	go func() {
		conn, _ := server.Accept()
		sockets <- conn
	}()
	client, err := net.Dial("tcp", "localhost:5000")
	if err != nil {
		t.Errorf("could not connect test client to localhost:5000")
	}
	defer client.Close()
	server_side := <- sockets
	thing1 := Thingy {
		Name:	"test",
		ID:		1,
	}
	thing2 := Thingy {
		Name:	"hellow, world",
		ID:		2,
	}
	thing3 := Thingy {
		Name:	"😃",
		ID:		3,
	}
	bytes1, err := format(thing1, &type_store)
	bytes2, err := format(thing2, &type_store)
	bytes3, err := format(thing3, &type_store)
	if err != nil {
		t.Errorf("error formatting thing")
	}
	server_side.Write(bytes1)
	server_side.Write(bytes2)
	server_side.Write(bytes3)
	iface, err := nextStruct(client, &type_store)
	if err != nil {
		t.Errorf("nextStruct returned an error: %s", err)
	}
	if restored_thing, ok :=  iface.(*Thingy); ok {
		if restored_thing.Name != thing1.Name {
			t.Errorf("thingy from nextStruct doesn't have same Name")
		}
		if restored_thing.ID != thing1.ID {
			t.Errorf("thingy from nextStruct doesn't have same ID")
		}
	} else {
		t.Errorf("nextStruct did not return an interface which could be asserted as Thingy")
	}
	iface, err = nextStruct(client, &type_store)
	if err != nil {
		t.Errorf("nextStruct returned an error: %s", err)
	}
	if restored_thing, ok :=  iface.(*Thingy); ok {
		if restored_thing.Name != thing2.Name {
			t.Errorf("thingy from nextStruct doesn't have same Name")
		}
		if restored_thing.ID != thing2.ID {
			t.Errorf("thingy from nextStruct doesn't have same ID")
		}
	} else {
		t.Errorf("nextStruct did not return an interface which could be asserted as Thingy")
	}
	iface, err = nextStruct(client, &type_store)
	if err != nil {
		t.Errorf("nextStruct returned an error: %s", err)
	}
	if restored_thing, ok :=  iface.(*Thingy); ok {
		if restored_thing.Name != thing3.Name {
			t.Errorf("thingy from nextStruct doesn't have same Name")
		}
		if restored_thing.ID != thing3.ID {
			t.Errorf("thingy from nextStruct doesn't have same ID")
		}
	} else {
		t.Errorf("nextStruct did not return an interface which could be asserted as Thingy")
	}
}

func TestNextStructErrorWithBrokenSocket(t *testing.T) {
	type_store := NewTypeStore()
	type_store.AddType(reflect.TypeOf(Thingy{}), BuildThingy)
	sockets := make(chan net.Conn, 1)
	server, err := net.Listen("tcp", "localhost:5000")
	if err != nil {
		t.Errorf("could not start test server on localhost:5000")
	}
	defer server.Close()
	go func() {
		conn, _ := server.Accept()
		sockets <- conn
	}()
	client, err := net.Dial("tcp", "localhost:5000")
	if err != nil {
		t.Errorf("could not connect test client to localhost:5000")
	}
	client.Close()
	_, err = nextStruct(client, &type_store)
	if err == nil {
		t.Errorf("nextStruct did not return an error when the socket was closed")
	}
}

func TestNextStructNilWhenMissing(t *testing.T) {
	type_store := NewTypeStore()
	type_store.AddType(reflect.TypeOf(Thingy{}), BuildThingy)
	sockets := make(chan net.Conn, 1)
	server, err := net.Listen("tcp", "localhost:5000")
	if err != nil {
		t.Errorf("could not start test server on localhost:5000")
	}
	defer server.Close()
	go func() {
		conn, _ := server.Accept()
		sockets <- conn
	}()
	client, err := net.Dial("tcp", "localhost:5000")
	if err != nil {
		t.Errorf("could not connect test client to localhost:5000")
	}
	defer client.Close()
	server_side := <- sockets
	thing := Thingy {
		Name:	"test",
		ID:		1,
	}
	bytes, err := format(thing, &type_store)
	if err != nil {
		t.Errorf("error formatting thing")
	}
	server_side.Write(bytes)
	empty_store := NewTypeStore()
	iface, err := nextStruct(client, &empty_store)
	if iface != nil {
		t.Errorf("nextStruct returned something that wasn't in the passed type store")
	}
}
