# Chitin format specification v1 DRAFT

Chitin is a data serialization format. See https://chitin.io/ for
introduction and motivation.

**DRAFT VERSION**

# Definitions {#definitions}

## `varuint` {#varuint}

Variable-length unsigned integer encoding. Defined in
[SQLite's Variable-Length Integers](http://www.sqlite.org/src4/doc/trunk/www/varint.wiki).

This is **not** the
[Protocol Buffer varint format](https://developers.google.com/protocol-buffers/docs/encoding#varints).

## `varsint` {#varsint}

Variable-length signed integer encoding. Signed integers are converted
to unsigned integers as per
[ZigZag encoding](https://developers.google.com/protocol-buffers/docs/encoding#signed-integers),
and then encoded as [`varuint`](#varuint).


# Types {#types}

**TODO move uint8 etc here**: define types just once, then talk about
encoding them for slots and fields


# Frames, Envelopes and Messages {#f-e-m}

<img src="layout.svg"
  alt="Diagram of frames, envelopes, messages, slots and fields"
  style="width:90%; margin:1em;"/>

**Frames**: To split a stream (for example data read from a TCP
socket, HTTP request body, etc) into messages, we *frame* the data. A
frame is simply a length prefix. The prefix is encoded as
[varuint](#varuint), typically as a single byte. Frames are not
interleaved. When the size is implied by the container (for example,
key-value store value as a whole), a frame is not needed.

**Envelopes**: When we may see multiple different kinds of messages
(and different versions of how a message may be laid out also count as
different kinds), we need something to differentiate the message type.
A leading [varuint](#varuint) stores that information. In the schema,
an envelope is a map of unsigned integers to message versions.

**Messages**: This is the actual payload transported. Message contents
are defined by a *versioned* *schema* that both describes the content
of the message and selects the exact *wire format*.

Programs using Chitin can use each layer directly, based on their
requirements. The layers are independent: a stream of Messages of the
same type can be sent with just Frames, without Envelopes.

Every Message does know its size, but only after reading its Fields,
whereas a Frame knows its size up front. If messages are consumed
fully and sequentially, Frames may be unnecessary. Similarly, when
consuming data fully, if unknown Messages in an Envelope are a fatal
error, sequences of Envelopes do not necessarily need Frames.
Recommendation: If the data size is unknown (a stream), always use
Frames, but write Chitin encoding/decoding libraries without such
assumptions.


# Frame {#frame}

A Frame is encoded as

- `varuint`: length of content
- `[n]byte`: content

Length 0 means skip this frame silently, and is used for alignment.

Libraries MUST allow applications to constrain maximum frame size.

The data type of the length is explicitly not specified as any fixed
size integer. Implementations can pick a size based on what is their
supported maximum frame size. Requirement to send or receive frames
greater than 4GB SHOULD be explicitly stated in any protocol
documentation if assumed, and not all implementations will be able to
do so. Library implementors SHOULD NOT choose a size smaller than
`uint32` unless working in very constrained environments.


# Envelope {#envelope}

An Envelope is encoded as

- `varuint`: message kind
- Message

Kind 0 means skip this envelope silently, and is used for alignment.

As with Frames, the data type of kind is explicitly not specified.
Implementations may look at the schema for the envelope and choose a
suitable size for the maximum number present. Implementations MUST
gracefully handle input greater than the chosen size.


# Message wire format v1 {#message-wire-v1}

A *Message* is a sequence of fixed-size *Slots*, a sequence of
variable-size *Fields*, and a sequence of the respective field
lengths.

Slots and fields of a message are explicitly described by the
versioned message definition in the schema. All slots and fields are
required; the closest a field can be to optional is that its length
may be 0, e.g. making a string be empty.

Libraries typically offer mechanisms to translate system and
application types into these types, for things like timestamps.
(**TODO schema should be able to talk about more abstract types, e.g.
to say "this is nanoseconds since unix epoch"**)

We will use the following short hand notation:

- `S`: a placeholder for any slot type
- `F`: a placeholder for any field type
- `M`: a placeholder for any message type
- `T`: a placeholder for any type that is valid in that context: `S`
  when talking about slots, `F` when talking about fields
- `[n]T`: a fixed-size array of `n` items of type `T`
- `[]T` for a variable-size array of items of type `T`

Message schema can specify a minimum alignment for the message.


## Slot {#slot}

A Slot contains one of the following data types:

- `uint8`, `uint16`, `uint32`, `uint64`
- `int8`, `int16`, `int32`, `int64`
- `float32`, `float64`
- (**TODO convenience support for bit maps, flags**)
- `byte`: like `uint8`, but with a semantic hint that it's not used as a number
- `M`: messages that do not contain fields
- `[n]S`: fixed length arrays of any of these types

Single-byte fields are encoded as-is.

Multi-byte numbers are encoded in big endian, to allow using messages
as lexicographical sort keys.

Floats are encoded as per IEEE-754.

Note that arrays of arrays are supported.

Slots are not aligned automatically. Given a sufficient alignment
guarantee set on the containing message, you can arrange slots to
provide the desired layout. Padding is explicit, and done with fields
named `_`.


## Field {#field}

A Field contains one the following data types:

- `uint8`, `uint16`, `uint32`, `uint64`
- `int8`, `int16`, `int32`, `int64`
- `float32`, `float64`
- `byte`: like `uint8`, but with a semantic hint that it's not used as a number
- (**TODO convenience support for bit maps, flags, combinations**)
- `[n]S`: fixed length arrays of any of fixed-size types
- `[]S`: variable length arrays of fixed-size types
- `string`: like `[]byte`, but with a semantic hint that it contains
  human-readable text in UTF-8 encoding
- `M`: messages

Single-byte integers, floats, fixed length arrays, messages that do
not contain fields, and fixed length arrays of fixed length types are
encoded as in slots. These fields do not store a field length.

All other field types have their length stored separately, with the
field content being variable length.

Multi-byte integers are encoded as [`varuint`](#varuint) or
[`varsint`](#varsint), respectively.

Note that arrays of arrays are supported.

Arrays with variable-size items can be stored by storing *Framed*
messages. In that case, constant-time lookup by index is not
supported.

Maps (aka dictionaries) of fixed-size keys and values can be stored as
an array of messages, with the message having slots for key and value.
Constant-time lookup by key is not supported, only by index.

Maps of variable-size items can be stored similarly using the above
method for storing arrays of variable-size items.

Message schema can specify a minimum alignment for a field.


## Storing field lengths {#field-len}

The length of a field always precedes the field content, but length
chunks and fields are interleaved to serve both streaming generation
and streaming consumption use cases, and to make use of the space
otherwise wasted as padding for alignment.

If a field length takes more than one byte to encode, the encoded
length may span multiple chunks. The field length will be always fully
transmitted before the field content begins; as a corner case,
multiple field length chunks may be sent back to back.

*(The constants in the following may need to be adjusted while this
spec is still a draft. Every mention of a constant in the spec must
specify a name for the constant.)*

Field lengths are encoded in chunks. Each chunk is up to 4 bytes
(constant `MaxChunkLen`) long.


## Alignment {#alignment}

Schema can specify a minimum alignment for a message, and fields of a
message. This allows for efficient processing of the data, e.g.
avoiding alignment faults and enabling special instructions to be
used.

When decoding a buffer, alignment can only be guaranteed if the buffer
is aligned according to the largest alignment guarantee that can be
contained in the buffer. When decoding a stream, the byte position
where reading starts is interpreted as offset 0; if part of the stream
has already been consumed, this may not match with any file offsets.

Libraries allocating buffers MUST either align buffers appropriately,
or state they do not support alignment.

Messages with no Envelope or Frame around them are correctly aligned
by the above.

Messages in Frames, either with or without Envelopes, are aligned by
prefixing 0-length Frames as appropriate. Note that the encoding of
the intermediate Envelope affects the amount of padding required.

Messages in Envelopes, without Frames, are aligned by prefixing
Envelopes with kind 0.

(Pragmatically, with and without Frames, the data is zero-prefixed,
but a Frame can only contain one Envelope or Message.)


# Schema {#schema}

**TODO**

**TODO how do slots & fields refer to messages**


## Schema Data Model {#schema-data-model}

**TODO**

## Schema Language {#schema-language}

**TODO**


# Rationale {#rationale}

*This section is non-normative.*

## varuint

We use
[SQLite's Variable-Length Integers](http://www.sqlite.org/src4/doc/trunk/www/varint.wiki),
opting to call them `varuint` inspired by
[github.com/dchest/varuint](https://github.com/dchest/varuint), to
encode integers in to least possible number of bytes.

`varuint` wins over Protocol Buffers' zigzag encoding for numbers in
the 128-240 range, which is very common for message field lengths.

`varuint` numbers sort properly in lexicographic order.

`varuint` puts the length of the encoded data in the first byte, a
property Protocol Buffers authors have said they would use, except for
legacy reasons.


## Interleaved field length chunks and contents

TODO
