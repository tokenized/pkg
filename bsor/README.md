# Bitcoin Script Object Representation (BSOR)

Bitcoin script object representation is a method for encoding predefined objects in Bitcoin script. It is similar to [CBOR (Concise Binary Object Representation)](https://cbor.io/) and [Protocol Buffers](https://developers.google.com/protocol-buffers) in that it precedes each element with an identifier.


## Encoding

Encoding depends on the fields being defined in advance. It is not possible to parse data without the field definitions. This is so that type information doesn't need to be encoded and the space can be saved.

### Field Count

The first value of any object encoding is a field count that specifies how many fields are encoded. The field count is encoded as a Bitcoin script number.

### Field

Each field is self encapsulated and the exact format depends on the type of the field.

#### Field Identifier

The first value of each field is the identifier for the field. The identifier is an integer that is unique to the object. The identifiers are Bitcoin script numbers.

The remaining encoding for the field is specific to the data type of the field.

#### Field Types

Field types are boolean, integer, string, binary, float, array, or another object.

Field types are defined in advance and the type is determined by knowing the object definition and using the field identifier.

Fields that are the "zero" value are excluded from encoding to save space. For example an empty array, false boolean, or zero integer will not be encoded.

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

The first value, after the field identifier, of an array is the number of items in the array encoded as a Bitcoin script number.

Then the specified number of items encoded as defined in advance.

##### Object

Fields can also be objects with their own set of fields.

The first value, after the field identifier, of an object is the number of fields encoded for the object, encoded as a Bitcoin script number.

After the specified number of fields are consumed, then the next field, if there are any, belongs to the parent object.

## Terms

### Bitcoin Script Number

Bitcoin script numbers are either a number op code like `OP_1` through `OP_16`, `OP_1NEGATE`, or a little endian encoded integer in a push data.
