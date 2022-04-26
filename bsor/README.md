# Bitcoin Script Object Representation (BSOR)

Bitcoin script object representation is a method for encoding predefined objects in Bitcoin script. It is similar to [CBOR (Concise Binary Object Representation)](https://cbor.io/) and [Protocol Buffers](https://developers.google.com/protocol-buffers) in that it precedes each field with an identifier.

## Usage

### Marshalling

To marshal a structure into BSOR call the `bsor.Marshal` function. It will return a list of `bitcoin.ScriptItem`. These items represent either op codes or push datas. Call `scriptItems.Script` to convert this to a Bitcoin script, which is an array of bytes that can be put in a Bitcoin transaction.

### Unmarshalling

To unmarshal a BSOR script into a structure call the `bsor.Unmarshal` function with the script items and a pointer to the structure. To get the script items from the script call `bitcoin.ParseScriptItems`. `bsor.Unmarshal` will populate the structure's fields and return any script items remaining.

## Encoding

Encoding depends on the fields being defined in advance. It is not possible to parse data without the field definitions. This is so that type information doesn't need to be encoded and the space can be saved.

BSOR simply precedes each field with an integer ID and provides some field counts. That with a predefined structure and field identifiers allows very light weight and efficient data. CBOR does nearly the same thing, but not in Bitcoin script encoding. Using Bitcoin script provides many advantages, some probably not known yet. One of which is it makes it easier to filter for specific data. CBOR requires first decoding the CBOR push data before applying a filter.

### Field Count

The first value of any object encoding is a field count that specifies how many fields are encoded. The field count is encoded as a Bitcoin script number.

### Field

Each field is self encapsulated and the exact format depends on the type of the field.

#### Field Identifier

The first value of each field is the identifier for the field. The identifier is a non-zero integer that is unique to the object. The identifiers are Bitcoin script numbers.

The remaining encoding for the field is specific to the data type of the field.

#### Field Types

Field types are boolean, integer, string, binary, float, array, or another object.

Field types are defined in advance and the type is determined by knowing the object definition and using the field identifier.

Fields that are the "zero" value are excluded from encoding to save space. This includes nil/null pointers, false booleans, zero value numbers, empty strings or bytes, and empty arrays. When encoding those field identifiers are not included, so when decoding if they are missing then the field is defaulted to its "zero" value.

##### Nil/Null

If the field is a pointer then the value can be represented a a nil with a zero. If it is a struct then the field count is just set to zero. If it is a primitive then each pointer value is preceded by a push op of 1 and a nil is represented by a push op of zero.

##### Boolean

Boolean fields are encoded as integers. True is `OP_1` by default, but as in Bitcoin script, any non-zero integer value should be evaluated as true.

##### Integer

Integers are encoded as Bitcoin script numbers.

##### String

Strings are UTF-8 text characters in a push data.

##### Binary

Binary data is encoded as bytes in a push data.

##### Float

Floats are either 32 or 64 bit. 32 bit floats are encoded as a 4 byte push data and 64 bit floats are encoded as an 8 byte push data.

##### Array

An array can contain any of the other field types and that type is fixed and must be defined in advance.

The first value of an array, after the field identifier, is the number of items in the array encoded as a Bitcoin script number.

Then the specified number of items encoded as defined in advance.

##### Fixed Size Array

A fixed size array is the same as an array except the number of items is not encoded as it is predefined by the structure definition.

##### Object

Fields can also be objects with their own set of fields.

The first value, after the field identifier, of an object is the number of fields encoded for the object, encoded as a Bitcoin script number.

After the specified number of fields are consumed, then the next field, if there are any, belongs to the parent object.

## Terms

### Bitcoin Script Number

Bitcoin script numbers are either a number op code like `OP_1` through `OP_16`, `OP_1NEGATE`, or a little endian encoded integer in a push data.

## Example

### Structure Definition

This implementation uses golang reflection and struct tags (decorators) so you can define a structure as follows simply by defining it in golang and including the `bsor` struct tags. More examples can be found in `bsor_test.go`.

```
type TestStructSimple struct {
	IntField            int               `bsor:"1"`
	StringField         string            `bsor:"2"`
	IntZeroField        int               `bsor:"3"`
	SubStruct           TestSubStruct     `bsor:"4"`
	BinaryField         []byte            `bsor:"5"`
	IntPointerField1    *int              `bsor:"6"`
	IntPointerField2    *int              `bsor:"7"`
	PublicKeyField      bitcoin.PublicKey `bsor:"8"`
	ArrayStringPtrField []*string         `bsor:"25"`
}

type TestSubStruct struct {
	SubIntField    int    `bsor:"1"`
	SubStringField string `bsor:"2"`
}

key, _ := bitcoin.GenerateKey(bitcoin.MainNet)
intValue := 102
stringValue := "string value"

value := TestStructSimple{
	IntField:    100,
	StringField: "test string",
	SubStruct: TestSubStruct{
		SubIntField:    101,
		SubStringField: "sub_string",
	},
	BinaryField:      []byte{0xab, 0xcd, 0xef},
	IntPointerField1: &intValue,
	PublicKeyField:   key.PublicKey(),
	ArrayStringPtrField: []*string{
		nil,
		&stringValue,
	},
}

scriptItems, err := bsor.Marshal(value)
if err != nil {
	return errors.Wrap(err, "bsor marshal")
}

script, err := scriptItems.Script()
if err != nil {
	return errors.Wrap(err, "create script")
}

fmt.Printf("Script : %s", script)

readScriptItems, err := bitcoin.ParseScriptItems(bytes.NewReader(script), -1)
if err != nil {
	return errors.Wrap(err, "parse script")
}

readValue := &TestStructSimple{}
remainingScriptItems, err := bsor.Unmarshal(readScriptItems, readValue)
if err != nil {
	return errors.Wrap(err, "unmarshal script")
}
```

### Script

The above structure encodes to this text representation:

`OP_7 OP_1 0x64 OP_2 "test string" OP_4 OP_2 OP_1 0x65 OP_2 "sub_string" OP_5 0xabcdef OP_6 0x66 OP_8 0x02d28913cf1fd781944fe3580f8a6fd93ea1427d8bd8bcd6106229ec4cd6c09b3e 0x19 OP_2 OP_0 OP_1 "string value"`

Or the raw script which is 95 bytes (hex representation of binary data):

`57510164520b7465737420737472696e675452510165520a7375625f737472696e675503abcdef560166582102d28913cf1fd781944fe3580f8a6fd93ea1427d8bd8bcd6106229ec4cd6c09b3e01195200510c737472696e672076616c7565`

This script should most likely be embedded in an OP_RETURN or other non-executing script when it is embedded in a Bitcoin transaction as it simply pushes things onto the stack. It should also be preceded by something that identifies the exact structure to be decoded. This is most commonly a protocol identifier and a message type identifier.

### Encoding

`OP_7` - The encoding for a structure starts with the number of fields encoded for the structure. `OP_7` is the single byte op code for the integer 7.

#### IntField

`OP_1` - Each field starts with the the field identifier specified in the `bsor` struct tag. `OP_1` is the single byte op code for the integer 1 which corresponds to the field `IntField`.

`0x64` - Integers are encoded as Bitcoin Script numbers. 0 through 16 can be encoded with the single byte op codes `OP_0` through `OP_16`, but higher numbers are encoded with raw binary push data in little endian. `0x64` is push data represented in hex, representing the integer value 100.

#### StringField

`OP_2` - The next field has an ID of 2.

`"test_string"` - Text strings and byte slices (variable sized arrays) are encoded as push datas containing the data. Note that the quotes are only for the text representation of the script. Only the 11 characters for the text are included in the push data.

#### IntZeroField

Note that there is no `OP_3` following the `StringField`. Since the integer's value is zero it is not included in the encoding.

#### SubStruct

`OP_4` - The next encoded field has an ID of 4.

`OP_2` - Structure definitions start with the number of fields that are encoded for the structure. This one has two fields encoded.

##### SubStruct.SubIntField

`OP_1` - The first field encoded for `SubStruct` is `SubIntField`.

`0x65` - The integer is encoded as a push data containing the byte `0x65` which is the integer value 101.

##### SubStruct.SubStringField

`OP_2` - The second field encoded for `SubStruct` is `SubStringField`.

`"sub_string"` - The value of the string is encoded as a push data containing the ASCII characters "sub_string".

Since `SubStruct` specified it had 2 fields encoded we now know that `SubStruct` is complete and we are back in the parent structure.

#### BinaryField

`OP_5` - The next encoded field for the top level structure is `BinaryField` with the id of 5.

`0xabcdef` - Byte slice fields are encoded as push datas. This is a push data containing the bytes represented by the hex `0xabcdef`.

#### IntPointerField1

`OP_6` - The next encoded field is `IntPointerField1` with the id of 6.

`0x66` - The value of the integer is `0x66` hex or 102.

#### IntPointerField2

Since `IntPointerField2` is nil the field is not encoded and the next id is `OP_8`.

#### PublicKeyField

`OP_8` - The next encoded field is `PublicKeyField` with the id of 8.

`0x02bffc5ea3d537625f70f8962d9bb52f1fbb38b4f5e83d2e9427545ca322364542` - Public keys have the golang `encoding.BinaryMarshaler` interface implemented that specifies how they are marshalled for binary encodings. A public key is marshalled as 33 bytes, the first of which is 0x02 or 0x03. This allows objects that already have well known binary encodings to retain those and not need BSOR identifiers to be defined.

#### ArrayStringPtrField

`0x19` - The next encoded field is `ArrayStringPtrField` with the id of 25, which is too large for a single byte integer op code so it is a push data containing the byte 0x19 which is the hex representation for the byte value 25.

`OP_2` - Variable sized arrays specify the number of items in the array immediately after the field id. Note that fixed sized arrays do not encode the number of items because it is defined in the structure.

`OP_0` - When an array contains pointers that can be nil a bool must precede the encoding of each item to specify if the field is nil or not. `OP_0` means the first item in this array is nil.

`OP_1` - Since the first item was nil there is not encoded value. `OP_1` is the start of the second item and specifies it is not nil.

`"string value"` - Since the second item is not nil its value is encoded after the `OP_1`. Since it is a string it is encoded as a push data containing the characters.
