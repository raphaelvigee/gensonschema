package tpl

import (
    "bytes"
    "fmt"
    "github.com/ohler55/ojg/gen"
    "github.com/ohler55/ojg/oj"
    "slices"
    "strconv"
    "sync/atomic"
    "unsafe"
    "github.com/ohler55/ojg/jp"
)

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
    ensureDataDeep(bool)
    result() any
	setv(any) error
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
        _path:   slices.Clone(node_path(from)).N(n),
        _parent: from,
        _ppath:  strconv.Itoa(n),
        _safe:   from._safe,
    }
}

func node_get[F, T __delegate](from *__node[F], path string) __node[T] {
    from.ensureData()

	return __node[T]{
        _data:   from._data,
        _path:   slices.Clone(node_path(from)).C(path),
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
	res, _ := r.result().([]any)

	return len(res)
}

func is_setting_array_index[RD __delegate](r *__node[RD]) bool {
	if len(r._path) >= 1 {
		maybeIndex := r._path[len(r._path)-1]

		_, ok := maybeIndex.(jp.Nth)

		return ok
    }

	return false
}

func node_array_set[RD __delegate](r *__node[RD], v any) error {
    index := r._path[len(r._path)-1].(jp.Nth)

    arr, _ := r._parent.result().([]any)
    if arr == nil {
        arr = make([]any, 0)
    }
    arr = append(arr, v)

    for i := len(arr); i <= int(index); i++ {
        arr = append(arr, nil)
    }

	arr[index] = v

    return r._parent.setv(arr)
}

func node_array_append_node[RD, VD __delegate](r *__node[RD], v *__node[VD]) error {
	return node_array_append(r, v.result())
}

func node_array_append(r __node_interface, v any) error {
	arr, _ := r.result().([]any)
	if arr == nil {
		arr = make([]any, 0)
    }
    arr = append(arr, v)

	return r.setv(arr)
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

func (r *__node[D]) ensureDataDeep(self bool) {
	if !r.Exists() {
        if parent := r._parent; parent != nil {
            parent.ensureDataDeep(true)
        }

		if self {
            err := r.set(r.unsafeGetString(r.defaultJson()))
			if err != nil {
				panic(err)
            }
        }
    }
}

func (r *__node[D]) result() any {
    r.ensureData()

    // TODO: optimize to use parent cache
	res := node_path(r).Get(r._data._data)
	if len(res) == 0 {
        return nil
    }

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

func (r *__node[D]) ensureMap(v any) (any, error) {
    b, err := oj.Marshal(v)
    if err != nil {
        return nil, err
    }

    data, err := oj.Parse(b)
    if err != nil {
        return nil, err
    }

	return data, nil
}

func (r *__node[D]) setnode(v __node_interface) error {
    data, err := r.ensureMap(v)
	if err != nil {
		return err
    }

    return r.setv(data)
}

func (r *__node[D]) setv(incomingv any) error {
    r.ensureData()
	r.ensureDataDeep(false)

    if node_path(r).String() == jp.R().String() {
		r._data._data = incomingv
		r._data._c.Add(1)

		return nil
    }

	if is_setting_array_index(r) {
		return node_array_set(r, incomingv)
    } else {
        return node_path(r).SetOne(r._data._data, incomingv)
    }
}

func (r *__node[D]) setMerge(incoming any) error {
    data, err := r.ensureMap(incoming)
    if err != nil {
        return err
    }

    arr, ok := r.result().(map[string]any)
	if !ok {
		arr = make(map[string]any)
    }

	for k, v := range data.(map[string]any) {
		arr[k] = v
    }

	return r.setv(arr)
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
	s := d.typeDefaultJson()
	if len(s) == 0 {
		return []byte("{}")
    }
    return s
}
