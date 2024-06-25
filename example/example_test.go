//go:generate go run github.com/raphaelvigee/gensonschema/cmd
package gen_test

import (
	"encoding/json"
	gen "github.com/raphaelvigee/gensonschema/example"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

var sample []byte

func init() {
	sample = []byte(`{"hello": "world"}`)
}

func TestUnmarshalMarshal(t *testing.T) {
	var obj gen.Person
	err := json.Unmarshal(sample, &obj)
	require.NoError(t, err)

	actual, err := json.Marshal(obj)
	require.NoError(t, err)

	assert.JSONEq(t, string(sample), string(actual))
}

func TestGetSetProperty(t *testing.T) {
	var obj gen.Person
	err := json.Unmarshal(sample, &obj)
	require.NoError(t, err)

	value := obj.GetFirstName().Value()
	assert.Equal(t, "", value)

	err = obj.GetFirstName().Set("hello")
	require.NoError(t, err)

	value = obj.GetFirstName().Value()
	assert.Equal(t, "hello", value)

	actual, err := json.Marshal(obj)
	require.NoError(t, err)

	assert.JSONEq(t, `{"hello": "world", "firstName": "hello"}`, string(actual))
}

func TestGetSetOneOf(t *testing.T) {
	var obj gen.OneOfRootObj
	err := json.Unmarshal([]byte(`{"firstName": "Bob"}`), &obj)
	require.NoError(t, err)

	firstname := obj.AsPerson().GetFirstName().Value()
	assert.Equal(t, "Bob", firstname)

	err = obj.AsPerson().GetFirstName().Set("Alice")
	require.NoError(t, err)

	firstname = obj.AsPerson().GetFirstName().Value()
	assert.Equal(t, "Alice", firstname)

	actual, err := json.Marshal(obj)
	require.NoError(t, err)

	assert.JSONEq(t, `{"firstName": "Alice"}`, string(actual))
}

func TestSetOneOfRoot(t *testing.T) {
	var obj gen.OneOfRootObj
	err := json.Unmarshal([]byte(`{"firstName": "Bob"}`), &obj)
	require.NoError(t, err)

	var vehicle gen.Vehicle
	err = vehicle.GetBrand().Set("Mercedes")
	require.NoError(t, err)

	err = obj.AsVehicle().Set(&vehicle)
	require.NoError(t, err)

	actual, err := json.Marshal(obj)
	require.NoError(t, err)

	assert.JSONEq(t, `{"brand": "Mercedes"}`, string(actual))
}

func TestSetOneOf(t *testing.T) {
	var obj gen.OneOf
	err := json.Unmarshal([]byte(`{"data": {"firstName": "Bob"}}`), &obj)
	require.NoError(t, err)

	var vehicle gen.Vehicle
	err = vehicle.GetBrand().Set("Mercedes")
	require.NoError(t, err)

	err = obj.GetData().AsVehicle().Set(&vehicle)
	require.NoError(t, err)

	actual, err := json.Marshal(obj)
	require.NoError(t, err)

	assert.JSONEq(t, `{"data": {"brand": "Mercedes"}}`, string(actual))
}

func TestSetPrimitiveRoot(t *testing.T) {
	var str gen.String
	err := str.Set("hello")
	require.NoError(t, err)

	assert.Equal(t, "hello", str.Value())
}

func TestSetAllOf(t *testing.T) {
	var obj gen.AllOf

	err := obj.GetShipping_address().GetCity().Set("Paris")
	require.NoError(t, err)

	err = obj.GetShipping_address().GetType().Set("business")
	require.NoError(t, err)

	actual, err := json.Marshal(obj)
	require.NoError(t, err)

	assert.JSONEq(t, `{"shipping_address": {"city":"Paris", "type":"business"}}`, string(actual))

	shipAddress := obj.GetShipping_address()

	actual, err = json.Marshal(shipAddress)
	require.NoError(t, err)

	assert.JSONEq(t, `{"city":"Paris", "type":"business"}`, string(actual))

	err = shipAddress.Set(shipAddress)
	require.NoError(t, err)

	actual, err = json.Marshal(obj)
	require.NoError(t, err)

	assert.JSONEq(t, `{"shipping_address": {"city":"Paris", "type":"business"}}`, string(actual))
}

func TestAllOfOneOf(t *testing.T) {
	var obj gen.AllOfOneOf

	_ = obj.GetData().GetB()
	_ = obj.GetData().AsAllOf0OneOf0().GetA1()
	_ = obj.GetData().AsAllOf0OneOf1().GetA2()
	_ = obj.GetData().AsNamedOneOf0().GetC1()
	_ = obj.GetData().AsNamedOneOf1().GetC2()
	_ = obj.GetData().AsDNestedTitle1().GetD1()
	_ = obj.GetData().AsAllOf3OneOf1().GetD2()
}

func TestArray(t *testing.T) {
	var obj gen.ArrayArray
	err := json.Unmarshal([]byte(`{"topfield1": [{"field1": "hello"}]}`), &obj)
	require.NoError(t, err)

	value := obj.GetTopfield1().At(0).GetField1().Value()
	assert.Equal(t, "hello", value)

	_ = obj.GetTopfield1().At(0).GetField2()

	_ = obj.GetTopfield1().Clear()

	assert.Empty(t, obj.GetTopfield1().Len())

	_ = obj.GetTopfield2().Append("hello")

	assert.JSONEq(t, `["hello"]`, string(obj.GetTopfield2().JSON()))

	_ = any(obj.GetTopfield2().Value()).([]string)
}

func TestNestedArrays(t *testing.T) {
	var obj gen.NestedarraysNestedarrays

	_ = obj.GetField1().At(0).GetField2().At(0).GetField3()
}
