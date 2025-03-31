package tpl

import (
    "bytes"
    "sync/atomic"
    "unsafe"
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
    _json string
    _c atomic.Uint64
}

type __node_interface interface {
    currentJson() string
}

type __node[D __delegate] struct {
	_data *__data
	_path string

	_parent __node_interface
	_ppath string

	_rc uint64
	_rjson string

	_safe bool
}

func node_get[F, T __delegate](from __node[F], path string) __node[T] {
	return __node[T]{
        _data:   from._data,
        _path:   pathJoin(from._path, path),
        _parent: from,
        _ppath:  path,
        _safe:   from._safe,
    }
}

func node_get_as[F, T __delegate](r __node[F]) __node[T] {
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
	result() gjson.Result
}

func node_array_len(r __node_result) int {
    res := r.result()
    if !res.IsArray() { return 0 }
    return int(res.Get("#").Int())
}

func node_value_string[T __delegate](r __node[T]) string {
    v := r.result().String()
    if r._safe {
        v = strings.Clone(v)
    }

	return v
}

func node_value_struct[T any](r __node_result) T {
    res := r.result()
    var v T
    _ = json.Unmarshal([]byte(res.Raw), &v)
    return v
}

// https://www.reddit.com/r/golang/comments/14xvgoj/converting_string_byte/?utm_source=share&utm_medium=web3x&utm_name=web3xcss&utm_term=1&utm_content=share_button
func (r __node[D]) unsafeGetBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

func (r __node[D]) unsafeGetString(b []byte) string {
    return unsafe.String(unsafe.SliceData(b), len(b))
}

func (r __node[D]) currentJsonb() []byte {
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
}

func (r __node[D]) MarshalJSON() ([]byte, error) {
    return r.JSON(), nil
}

func (r __node[D]) JSON() []byte {
    return r.currentJsonb()
}

func (r __node[D]) withSafe(safe bool) __node[D] {
    r._safe = safe
    return r
}

func (r *__node[D]) newData(b string) *__data {
    return &__data{_json: b, _c: atomic.Uint64{}}
}

func (r *__node[D]) UnmarshalJSON(b []byte) error {
    if r._data != nil {
        if r._path == "" {
            r.setJson(r.unsafeGetString(b))
            return nil
        }

        njson, err := sjson.SetRaw(r.json(), r.path(), string(b))
        if err != nil {
            return err
        }
        r.setJson(njson)
        return nil
    }

    *r = __node[D]{_data: r.newData(r.unsafeGetString(b))}
    return nil
}

func (r __node[D]) json() string {
    if r._data == nil {
        return string(r.defaultJson())
    }

    return r._data._json
}

func (r __node[D]) path() string {
    return r._path
}

func (r __node[D]) setJson(v string) {
    r._data._json = v
	r._data._c.Add(1)
}

func (r *__node[D]) ensureJson() {
    if r._data != nil {
        return
    }

    b := r.json()
    r._data = r.newData(b)
}

func (r __node[D]) result() gjson.Result {
	if parent := r._parent; parent != nil {
        return gjson.Get(parent.currentJson(), r._ppath)
    }

    if r._path == "" {
        return gjson.Parse(r.json())
    }
    return gjson.Get(r.json(), r.path())
}

func (r __node[D]) Exists() bool {
    return r.result().Exists()
}

func (r __node[D]) Delete() error {
    res, err := sjson.Delete(r.json(), r.path())
    if err != nil {
        return err
    }
    r.setJson(res)
    return nil
}

func (r *__node[D]) setb(incoming []byte) error {
    return r.set(r.unsafeGetString(incoming))
}

func (r *__node[D]) set(incoming string) error {
    r.ensureJson()

    if r._path == "" {
        r.setJson(incoming)
        return nil
    }

    res, err := sjson.SetRaw(r.json(), r.path(), incoming)
    if err != nil {
        return err
    }
    r.setJson(res)
    return nil
}

func (r *__node[D]) setMerge(incoming string) error {
	current := r.currentJson()

	var buf bytes.Buffer
	buf.Grow(len(current)+len(incoming)+3)
	buf.WriteByte('[')
	buf.WriteString(current)
	buf.WriteByte(',')
	buf.WriteString(incoming)
	buf.WriteByte(']')

    incoming2 := gjson.GetBytes(buf.Bytes(), "@join").Raw

    return r.set(incoming2)
}

func (r __node[D]) copy() __node[D] {
    j := r.currentJson()

    return __node[D]{
        _data: r.newData(j),
        _safe: r._safe,
    }
}

func (r __node[D]) defaultJson() []byte {
    var d D
	return d.typeDefaultJson()
}
