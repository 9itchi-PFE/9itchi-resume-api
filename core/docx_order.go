package core

import "strings"

// ordered lets a layout write its sections in their natural order and emit them in the order the user chose:
// everything before begin() is the fixed header, each cut(key) closes one section.
type ordered struct {
	b     *strings.Builder
	head  string
	mark  int
	parts map[string]string
}

func newOrdered(b *strings.Builder) *ordered { return &ordered{b: b, parts: map[string]string{}} }

func (o *ordered) begin() {
	s := o.b.String()
	o.head, o.mark = s, len(s)
}

func (o *ordered) cut(key string) {
	s := o.b.String()
	o.parts[key] = s[o.mark:]
	o.mark = len(s)
}

// flush returns the header followed by the sections in order.
func (o *ordered) flush(order []string) string {
	var out strings.Builder
	out.WriteString(o.head)
	for _, k := range order {
		out.WriteString(o.parts[k])
	}
	return out.String()
}

// replace rewrites the builder with the reordered document.
func (o *ordered) replace(order []string) {
	s := o.flush(order)
	o.b.Reset()
	o.b.WriteString(s)
}
