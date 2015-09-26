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

**TODO convenience support for bit maps, flags**

## Fixed-width integers {#integers}

- `uint8`, `uint16`, `uint32`, `uint64`: unsigned integers of the
  given bit width
- `int8`, `int16`, `int32`, `int64`: signed integers of the given bit
  width


## Floating point numbers {#floats}

- `float32`: IEEE-754 32-bit floating-point numbers
- `float64`: IEEE-754 64-bit floating-point numbers


## Array types {#array-types}

- `[n]T`: a fixed-length array of `n` items of type `T`
- `[]T`: a variable-length array of items of type `T`

Where `T` is any type that is valid in that context.


## Aliases {#alias-types}

- `byte`: like `uint8`, but with a semantic hint that it's not used as
  a number
- `string`: like `[]byte`, but with a semantic hint that it contains
  human-readable text in UTF-8 encoding


# Length-prefixed encoding {#length-prefixed-encoding}

This section describes an encoding for sequences of variable-length
items.

In its basic form, the encoding simply alternates `varuint` lengths
with the full content of one item.

Length 0 is reserved for use as padding (see [Alignment](#alignment)).
Encoded length is content length + 1.

> *Example*: an encoding of two variable-length messages with contents
> `x` and `foo`:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5
> -------|---|---|---|---|---|---
> value  | 2 |"x"| 4 |"f"|"o"|"o"

> *Example*: an encoding of a variable-length message with a content of
>  the letter `x` repeated 1000 times:
>
> offset | 0 | 1 | 2 | 3 | … | 1001
> -------|---|---|---|---|---|-----
> value  |243|249|"x"|"x"| … |"x"


## Alignment {#alignment}

The content may have alignment requirements. This allows for efficient
processing of the data, e.g. avoiding alignment faults and enabling
special instructions to be used.

When decoding a buffer, alignment can only be guaranteed if the buffer
is aligned according to the largest alignment guarantee that can be
contained in the buffer. When decoding a stream, the byte position
where reading starts is interpreted as offset 0; if part of the stream
has already been consumed, this may not match with any file offsets.

Libraries allocating buffers MUST either align buffers appropriately,
or state they do not support alignment.

In the simple case, alignment is implemented by one or more null bytes
(value `0`) after the length.

When decoding, the 0 lengths are skipped.

> *Note*: this only works because the decoder also knows the alignment
> requirements. Otherwise, the padding bytes would be confused for
> content.

> *Example*: an encoding of a variable-length message with contents
> `foo`, where the content is aligned to a 4-byte boundary:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5 | 6
> -------|---|---|---|---|---|---|---
> value  | 4 | 0 | 0 | 0 |"f"|"o"|"o"

The number of padding bytes depends on the encoded byte count of the
`varuint` length.

> *Example*: an encoding of a variable-length message with a content of
>  the letter `x` repeated 1000 times, where the content is aligned to a
>  4-byte boundary:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5 | … | 1003
> -------|---|---|---|---|---|---|---|-----
> value  |243|249| 0 | 0 |"x"|"x"| … |"x"

> *Example*: an encoding of two variable-length messages with contents
> `x` and `foo`, where `foo` is aligned to a 4-byte boundary:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5 | 6
> -------|---|---|---|---|---|---|---
> value  | 2 |"x"| 4 | 0 |"f"|"o"|"o"

With nested data structures, alignment may be required for a byte that
is not the first byte of content.

> *Example*: an encoding of a variable-length message with contents
> `xAbc`, where the byte `A` is aligned to a 4-byte boundary:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5 | 6
> -------|---|---|---|---|---|---|---
> value  | 5 | 0 | 0 |"x"|"A"|"b"|"c"


## Interleaving fixed length items {#interleaving-fixed-length-items}

Fixed length items may be interleaved with length-prefixed items. This
is only possible when the decoder will know to expect this.

> *Example*: an encoding of a variable-length message `foo`,
> fixed-length message `x`, and a variable-length message `quux`:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9
> -------|---|---|---|---|---|---|---|---|---|---
> value  | 4 |"f"|"o"|"o"|"x"| 5 |"q"|"u"|"u"|"x"

Alignment of fixed length items works as above, by inserting 0
lengths as needed.

> *Example*: an encoding of a variable-length message `foobar`,
> fixed-length message `x` aligned at a 4-byte boundary, and a
> variable-length message `quux`:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10| 11| 12| 13
> -------|---|---|---|---|---|---|---|---|---|---|---|---|---|---
> value  | 4 |"f"|"o"|"o"|"b"|"a"|"r"| 0 |"x"| 5 |"q"|"u"|"u"|"x"


## Padding optimization {#padding-optimization}

To minimize the overhead from padding, we can use the padding bytes to
encode upcoming item lengths.

Instead of padding with null bytes, encoder MAY use bytes from the
lengths of the items sequentially after the current item. Decoders
MUST support this.

> *Example*: an encoding of three variable-length messages with
> contents `x`, `foo` and `y`, where `foo` is aligned to a 4-byte
> boundary:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7
> -------|---|---|---|---|---|---|---|---
> value  | 2 |"x"| 4 | 2 |"f"|"o"|"o"|"y"

The bytes of a `varuint`-encoded length MAY be split on two sides of
an items content.

> *Example*: an encoding of three variable-length messages with
> contents `x`, `foo` and the letter `y` repeated 1000 times, where
> `foo` is aligned to a 4-byte boundary:
>
> offset | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | … | 1007
> -------|---|---|---|---|---|---|---|---|---|---|---|-----
> value  | 2 |"x"| 4 |243|"f"|"o"|"o"|249|"y"|"y"| … |"y"

Null byte padding MUST NOT be inserted in the middle of a
`varuint`-encoded length.


# Frames, Envelopes and Messages {#f-e-m}

<img src="layout.svg"
  alt="Diagram of frames, envelopes, messages, slots and fields"
  style="width:90%; margin:1em;"/>

**Frames**: To split a stream (for example data read from a TCP
socket, HTTP request body, etc) into messages, we *frame* the data. A
frame is simply a length prefix. The prefix is encoded as
[varuint](#varuint), typically as a single byte. Frames are not
interleaved. When the length is implied by the container (for example,
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

Every Message does know its length, but only after reading its Fields,
whereas a Frame knows its length up front. If messages are consumed
fully and sequentially, Frames may be unnecessary. Similarly, when
consuming data fully, if unknown Messages in an Envelope are a fatal
error, sequences of Envelopes do not necessarily need Frames.
Recommendation: If the data length is unknown (a stream), always use
Frames, but write Chitin encoding/decoding libraries without such
assumptions.


# Frame {#frame}

A Frame is encoded as

- `varuint`: length of content
- `[n]byte`: content

Length 0 means skip this frame silently, and is used for alignment.

Libraries MUST allow applications to constrain maximum frame length.

The data type of the length is explicitly not specified as any fixed
size integer. Implementations can pick a size based on what is their
supported maximum frame length. Requirement to send or receive frames
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
suitable integer size for the maximum number present. Implementations
MUST gracefully handle input greater than the chosen size.


# Message wire format v1 {#message-wire-v1}

A *Message* is a sequence of fixed-length *Slots* followed by a
sequence of variable-length *Fields*.

Slots and fields of a message are explicitly described by the
versioned message definition in the schema. All slots and fields are
required; the closest a field can be to optional is that its length
may be 0, e.g. making a string be empty.

Libraries typically offer mechanisms to translate system and
application types into these types, for things like timestamps.
(**TODO schema should be able to talk about more abstract types, e.g.
to say "this is nanoseconds since unix epoch"**)

We will use the following short hand notation:

- `M`: a placeholder for any message type
- `S`: a placeholder for any slot type
- `F`: a placeholder for any field type
- `T`: a placeholder for any type that is valid in that context: `S`
  when talking about slots, `F` when talking about fields

Message schema can specify a minimum alignment for the message.

Messages with no Envelope or Frame around them are correctly aligned
by relying of aligned buffer allocation (see [Alignment](#alignment)).

Messages in Frames, either with or without Envelopes, are aligned by
prefixing 0-length Frames as appropriate. Note that the encoding of
the intermediate Envelope affects the amount of padding required.

Messages in Envelopes, without Frames, are aligned by prefixing
Envelopes with kind 0.

(Pragmatically, with and without Frames, the data is zero-prefixed,
but a Frame can only contain one Envelope or Message.)


## Slot {#slot}

A Slot contains one of the following data types:

- `uint8`, `uint16`, `uint32`, `uint64`
- `int8`, `int16`, `int32`, `int64`
- `float32`, `float64`
- `byte`
- (**TODO convenience support for bit maps, flags**)
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

Fields can be by their very nature variable length. There are three
cases of how the field length is known, which may combine in any
order, and are collectively encoded as specified in
[Interleaving fixed length items](#interleaving-fixed-length-items).

Message schema can specify a minimum alignment for a field.


### Fixed length fields

- `uint8`
- `int8`
- `byte`
- (**TODO convenience support for bit maps, flags, combinations**)
- `M`: messages that do not contain fields
- `[n]S`: fixed length arrays of fixed-length types

These are encoded as in slots. They do not encode a length prefix.
They are supported in fields mostly for completeness; they are
probably better off put in slots.


### Self-delimited fields

- `uint16`, `uint32`, `uint64`
- `int16`, `int32`, `int64`
- `float32`, `float64`

Encoded in a way that does not need a separate length prefix. These
are all short enough that having a separate length prefix would be
wasteful.

Multi-byte integers are encoded as [`varuint`](#varuint) or
[`varsint`](#varsint), respectively.

Floats are converted to integers as per IEEE-754, their bytes are
reordered so that exponent is in the least significant bits, and
encoded as `varuint`. This minimizes space used by smaller
numbers.[^varfloat]


### Length-prefixed fields

- `M`: messages that contain fields
- `[]S`: variable length arrays of fixed-length types
- `string`

These use [length-prefixed encoding](#length-prefixed-encoding) as
specified earlier.


### Encoding more complex types

Arrays of arrays are supported, but inner arrays must be fixed length.

Arrays with variable-length items can be stored by storing *Framed*
messages in a `[]byte`. In that case, constant-time lookup by index is
not supported.

Maps (aka dictionaries) of fixed-length keys and values can be stored
as an array of messages, with the message having slots for key and
value. Constant-time lookup by key is not supported, only by index.

Maps of variable-length items can be stored similarly using the above
method for storing arrays of variable-length items.


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


## Interleaved field lengths and contents

The [length-prefixed encoding](#length-prefixed-encoding) interleaves
lengths and contents, because it seems like the best trade-off.

Other length-first formats have experienced pains from this decision
too, e.g. there's been talk of a Protocol Buffers encoding
optimization where the outgoing message byte buffer is constructed
back-to-front, so the field sizes are more naturally known at the time
they need to be encoded.

Let's look at the alternatives.

If *all* lengths were up *front*:

- Assuming `varuint` encoding for lengths, we wouldn't know where to
  start writing the content of the first field until we know how we'll
  encode all of the lengths. (Fixed-size lengths would mitigate this,
  but be wasteful in other ways.)
- We'd need to compute sizes for potentially large, potentially
  on-demand generated contents -- and then remember the contents, for
  later use.
- We could not start streaming the first field until after we'd
  computed sizes for all fields.
- The same mechanism would not serve us for Frames, where future frame
  lengths may depend on user input etc.

If *all* lengths were at the *back*:

- We wouldn't know where to write them until we'd know the sizes of
  all of the fields. This is less of a problem than above, as we could
  buffer the relatively small field sizes elsewhere.
- We could not start processing any of the streamed fields until after
  we'd received them all, to see the sizes.
- The same mechanism would not serve us for Frames, where future frame
  lengths may depend on user input etc.

Field lengths could be *implicit*:

- Using something like
  [COBS](https://en.wikipedia.org/wiki/Consistent_Overhead_Byte_Stuffing),
  every field could be self-terminating.
- This means we don't need to know the size of a field before we start
  writing it. That's nice, but decode having to look at every byte of
  the field is not.
- For the purposes of the discussion here, this is pretty much
  equivalent to interleaving lengths and contents.

So, some sort of interleaving seems ideal.

Our desire to support alignment guarantees causes wasteful null
padding. [Padding optimization](#padding-optimization) minimizes that,
at the cost of complexity. If that complexity is demonstrated to be a
significant barrier, we'll remove the optimization.



## Null padding is not visible to applications

0-length Frames and 0-kind Envelopes are never exposed to the
application by a library. This is so that it's always safe to

- insert null padding into Framed connections to avoid idle disconnections
- choose whether to do [padding optimization](#padding-optimization)
  based on whether the size of the next field has already been
  computer or not
- to concatenate two Framed streams, using the padding to guarantee
  alignments in the second stream

And so on. If applications were to see the pure padding entries, their
behavior might change, even from just timing differences.


<!-- Footnotes

blackfriday adds a horizontal ruler above the footnotes.

blackfriday is finicky about footnotes contents, do not word wrap the
following lines.

-->

[^varfloat]: This idea came from "Floats are converted to IEEE754, byte reversed, then uvarint encoded." in https://github.com/sbunce/gosu
