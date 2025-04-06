package tpl

import (
    "bytes"
    "fmt"
    "github.com/ohler55/ojg/gen"
    "github.com/ohler55/ojg/oj"
    "reflect"
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
}

type __node_interface interface {
    ensureDataDeep(bool)
    result() any
	setv(any) error
    path() jp.Expr
	parent() __node_interface
}

type __node[D __delegate] struct {
	_data *__data
	_path jp.Expr

	_parent __node_interface
}

func node_path(from __node_interface) jp.Expr {
    if from.path() == nil {
		return jp.R()
    }

	return from.path()
}

func node_is_root(r __node_interface) bool {
	if len(r.path()) == 0 {
		return true
    }

	if len(r.path()) == 1 {
        _, ok := r.path()[0].(jp.Root)

		return ok
    }

	return false
}

func node_at[F, T __delegate](from *__node[F], n int) __node[T] {
    from.ensureData()

    return __node[T]{
        _data:   from._data,
        _path:   slices.Clone(node_path(from)).N(n),
        _parent: from,
    }
}

func node_get[F, T __delegate](from *__node[F], path string) __node[T] {
    from.ensureData()

	return __node[T]{
        _data:   from._data,
        _path:   slices.Clone(node_path(from)).C(path),
        _parent: from,
    }
}

func node_get_as[F, T __delegate](r *__node[F]) __node[T] {
    r.ensureData()

    return __node[T]{
        _data: r._data,
        _path: r._path,
        _parent: r._parent,
    }
}

func node_value_float(n __node_interface) float64 {
    v := reflect.ValueOf(n.result())
    if v.CanFloat() {
        return v.Float()
    }

    if v.CanInt() {
        return float64(v.Int())
    }

    if v.CanUint() {
        return float64(v.Uint())
    }

	return 0
}

func node_value_int(n __node_interface) int64 {
    v := reflect.ValueOf(n.result())
    if v.CanFloat() {
        return int64(v.Float())
    }

    if v.CanInt() {
        return v.Int()
    }

    if v.CanUint() {
        return int64(v.Uint())
    }

    return 0
}

func node_value_uint(n __node_interface) uint64 {
    v := reflect.ValueOf(n.result())
    if v.CanFloat() {
        return uint64(v.Float())
    }

    if v.CanInt() {
        return uint64(v.Int())
    }

    if v.CanUint() {
        return v.Uint()
    }

    return 0
}

func node_value_bool(n __node_interface) bool {
    v, _ := n.result().(bool)

    return v
}

func node_value_string(n __node_interface) string {
    v, _ := n.result().(string)

    return v
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

func is_setting_array_index(r __node_interface) bool {
	if len(r.path()) >= 1 {
		maybeIndex := r.path()[len(r.path())-1]

		_, ok := maybeIndex.(jp.Nth)

		return ok
    }

	return false
}

func node_array_set(r __node_interface, v any) error {
    index := r.path()[len(r.path())-1].(jp.Nth)

    arr, _ := r.parent().result().([]any)
    if arr == nil {
        arr = make([]any, 0)
    }

    for i := len(arr); i <= int(index); i++ {
        arr = append(arr, nil)
    }

	arr[index] = v

    return r.parent().setv(arr)
}

func node_array_append_node(r, v __node_interface) error {
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

func (r __node[D]) MarshalJSON() ([]byte, error) {
	return oj.Marshal(r.result(), &oj.Options{Sort: true})
}

func (r __node[D]) JSON() []byte {
    b, _ := r.MarshalJSON()
    return b
}

func (r __node[D]) withSafe(safe bool) __node[D] {
    return r
}

func (r *__node[D]) newData(b string) *__data {
    data, err := oj.ParseString(b)
	if err != nil {
		panic(err)
    }

    return &__data{_data: data}
}

func (r *__node[D]) UnmarshalJSON(b []byte) error {
    if r._data != nil {
		r.set(r.unsafeGetString(b))

        return nil
    }

    *r = __node[D]{_data: r.newData(r.unsafeGetString(b))}
    return nil
}

func (r __node[D]) path() jp.Expr {
    return r._path
}

func (r __node[D]) parent() __node_interface {
    return r._parent
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
    val, err := node_path(r).RemoveOne(r._data._data)
    if err != nil {
        return err
    }

    r._data._data = val

    return nil
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

    if node_is_root(r) {
		r._data._data = incomingv

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
	b, err := r.MarshalJSON()
	if err != nil {
		panic(err)
    }

    return __node[D]{_data: r.newData(r.unsafeGetString(b))}
}

func (r __node[D]) defaultJson() []byte {
    var d D
	s := d.typeDefaultJson()
	if len(s) == 0 {
		return []byte("{}")
    }
    return s
}
