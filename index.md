# Chitin -- gives structure <br/> to your squishy bits

Chitin is a wire format for your data. You could transfer Chitin
messages over a TCP/IP socket, or write them to a file.

Chitin is especially aimed at being able to give structure to data
without needing a parse and copy pass. For example, chitin format data
inside an `mmapped` file will be readable without extra copies.

Direct data access is made possible by embracing fixed size data. You
can think of Chitin messages like the IETF protocol diagrams of old,
except computer managed. This is in stark contrast to formats such as
Protocol Buffers, where the message is essentially a sequence of
variable-length type-length-value submessages, recursively, and most
processing would copy the content out into fixed-size data structures
linked with pointers.

A Chitin message can contain variable length data, but only after all
the fixed fields.

Chitin manages schema versioning and protocol evolution by versioning
whole messages. Chitin combines message type and version information
into a single identifier for efficiency.

Chitin is language-independent, but at this time only a Go library
exists. That library uses code generation to gain good performance.


**Status:** the [spec](/spec/v1/) is starting to shape up, just about
everything else is missing. Be patient, or join us to help out!
https://github.com/chitin-io/web


# Example of a schema

**THIS WILL CHANGE**

```chitin
chitin v1

envelope Bar {
	map {
		1: Person v1
	}
}

message Person v1 {
	wire format: v1
	options {
		align: 4
		# switches field lengths from varuint to exponential golomb coding
		field length encoding: exp-golomb
	}

	slots {
		age uint16
		_ [2]byte
		foo uint32
	}
	fields {
		name string
		website url
		data bytes {
			align: 8
		}
	}
}
```
