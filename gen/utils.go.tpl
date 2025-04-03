package tpl

import (
    "bytes"
    "fmt"
    "github.com/ohler55/ojg/gen"
    "github.com/ohler55/ojg/oj"
    "strconv"
    "sync/atomic"
    "unsafe"
    "github.com/ohler55/ojg/jp"
)

func pathJoin(p1, p2 string) string {
    if p1 == "" {
        return p2
    }

    return p1+"."+p2
}

type __delegate interface {
    typeDefaultJson() []byte
}

type __node_array[T any] interface {
    Len() int
    At(i int) T
}

type __data struct {
    _data any
    _c atomic.Uint64
}

type __node_interface interface {
    JSON() []byte
}

type __node[D __delegate] struct {
	_data *__data
	_path jp.Expr

	_parent __node_interface
	_ppath string

	_rc uint64
	_rjson string

	_safe bool
}

func node_path[F __delegate](from *__node[F]) jp.Expr {
    if from._path == nil {
		return jp.R()
    }

	return from._path
}

func node_at[F, T __delegate](from *__node[F], n int) __node[T] {
    from.ensureData()

    return __node[T]{
        _data:   from._data,
        _path:   node_path(from).N(n),
        _parent: from,
        _ppath:  strconv.Itoa(n),
        _safe:   from._safe,
    }
}

func node_get[F, T __delegate](from *__node[F], path string) __node[T] {
    from.ensureData()

	return __node[T]{
        _data:   from._data,
        _path:   node_path(from).C(path),
        _parent: from,
        _ppath:  path,
        _safe:   from._safe,
    }
}

func node_get_as[F, T __delegate](r *__node[F]) __node[T] {
    r.ensureData()

    return __node[T]{
        _data: r._data,
        _path: r._path,
        _parent: r._parent,
        _ppath: r._ppath,
        _safe: r._safe,
    }
}

func node_array_range[T any](r __node_array[T]) func(yield func(int, T) bool) {
    return func(yield func(int, T) bool) {
        l := r.Len()

        for i := 0; i < l; i++ {
            v := r.At(i)

            if !yield(i, v) {
                break
            }
        }
    }
}

type __node_result interface {
	result() any
}

func node_array_len(r __node_result) int {
    // TODO: optimize to use parent cache

	//jp.Length(jp.C(r._path))

	return 0

    /*res := r.result()
    if !res.IsArray() { return 0 }
    return int(res.Get("#").Int())*/
}

func node_value_string[T __delegate](r __node[T]) string {
    v, _ := r.result().(string)
    if r._safe {
        v = strings.Clone(v)
    }

	return v
}

func node_value_struct[T any](r __node_result) T {
    data := r.result()

    b, err := oj.Marshal(data)
    if err != nil {
        panic(err)
    }

    var v T
    _ = oj.Unmarshal(b, &v)

    return v
}

// https://www.reddit.com/r/golang/comments/14xvgoj/converting_string_byte/?utm_source=share&utm_medium=web3x&utm_name=web3xcss&utm_term=1&utm_content=share_button
func (r __node[D]) unsafeGetBytes(s string) []byte {
    return unsafe.Slice(unsafe.StringData(s), len(s))
}

func (r __node[D]) unsafeGetString(b []byte) string {
    return unsafe.String(unsafe.SliceData(b), len(b))
}

/*func (r __node[D]) currentJsonb() []byte {
	return r.unsafeGetBytes(r.currentJson())
}

func (r __node[D]) currentJson() string {
    if r._path == "" {
        return r.json()
    }

    if r._rjson != "" && r._rc > 0 && r._rc == r._data._c.Load() {
        return r._rjson
    }

    res := r.result()

	r._rc = r._data._c.Load()
	r._rjson = res.Raw

    return r._rjson
}*/

func (r __node[D]) MarshalJSON() ([]byte, error) {
	return oj.Marshal(r.result())
}

func (r __node[D]) JSON() []byte {
    b, _ := oj.Marshal(r.result())
    return b
}

func (r __node[D]) withSafe(safe bool) __node[D] {
    r._safe = safe
    return r
}

func (r *__node[D]) newData(b string) *__data {
    data, err := oj.ParseString(b)
	if err != nil {
		panic(err)
    }

    return &__data{_data: data, _c: atomic.Uint64{}}
}

func (r *__node[D]) UnmarshalJSON(b []byte) error {
    if r._data != nil {
		r.set(r.unsafeGetString(b))

        return nil
    }

    *r = __node[D]{_data: r.newData(r.unsafeGetString(b))}
    return nil
}

func (r __node[D]) Path() string {
    return r._path.String()
}

func (r *__node[D]) ensureData() {
    if r._data == nil {
        r._data = r.newData(string(r.defaultJson()))
    }
}

func (r *__node[D]) result() any {
	/*if parent := r._parent; parent != nil {
        return gjson.Get(parent.currentJson(), r._ppath)
    }*/

    r.ensureData()

	res := node_path(r).Get(r._data._data)
	if len(res) == 0 {
        return nil
    }

    // TODO: optimize to use parent cache
    return res[0]
}

func (r *__node[D]) Exists() bool {
    // TODO: optimize to use parent cache
    return node_path(r).Has(r._data._data)
}

func (r *__node[D]) Delete() error {
    // TODO: optimize to use parent cache
    return node_path(r).DelOne(r._data._data)
}

func (r *__node[D]) set(incoming string) error {
    incomingv, err:= oj.ParseString(incoming)
    if err != nil {
        return err
    }

	return r.setv(incomingv)
}

func (r *__node[D]) setv(incomingv any) error {
    r.ensureData()

    if node_path(r).String() == jp.R().String() {
		r._data._data = incomingv
		r._data._c.Add(1)

		return nil
    }

	// TODO: optimize to use parent cache
    return node_path(r).SetOne(r._data._data, incomingv)
}

func (r *__node[D]) setMerge(incoming any) error {
	return nil

	/*current := r.currentJson()

	var buf bytes.Buffer
	buf.Grow(len(current)+len(incoming)+3)
	buf.WriteByte('[')
	buf.WriteString(current)
	buf.WriteByte(',')
	buf.WriteString(incoming)
	buf.WriteByte(']')

    incoming2 := gjson.GetBytes(buf.Bytes(), "@join").Raw

    return r.set(incoming2)*/
}

func (r __node[D]) copy() __node[D] {
	return r
    /*j := r.currentJson()

    return __node[D]{
        _data: r.newData(j),
        _safe: r._safe,
    }*/
}

func (r __node[D]) defaultJson() []byte {
    var d D
    return d.typeDefaultJson()
}
