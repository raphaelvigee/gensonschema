package tpl

import (
    "bytes"
    "sync/atomic"
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

type __data struct {
    _json []byte
    _c atomic.Uint64
}

type __node_interface interface {
	JSON() []byte
}

type __node[D __delegate] struct {
	_data *__data
	_path string

	_parent __node_interface
	_ppath string

	_rc uint64
	_rjson []byte
}

func (r __node[D]) currentJson() []byte {
    if r._path == "" {
        return r.json()
    }

    if r._rjson != nil && r._rc > 0 && r._rc == r._data._c.Load() {
        return r._rjson
    }

    res := r.result()

	r._rc = r._data._c.Load()
	r._rjson = []byte(res.Raw)

    return r._rjson
}

func (r __node[D]) MarshalJSON() ([]byte, error) {
    return r.currentJson(), nil
}

func (r __node[D]) JSON() []byte {
    return r.currentJson()
}

func (r *__node[D]) newData(b []byte) *__data {
    return &__data{_json: b, _c: atomic.Uint64{}}
}

func (r *__node[D]) UnmarshalJSON(b []byte) error {
    if r._data != nil {
        if r._path == "" {
            bcopy := make([]byte, len(b))
            copy(bcopy, b)

            r.setJson(bcopy)
            return nil
        }

        njson, err := sjson.SetRawBytes(r.json(), r.path(), b)
        if err != nil {
            return err
        }
        r.setJson(njson)
        return nil
    }

    bcopy := make([]byte, len(b))
    copy(bcopy, b)

    *r = __node[D]{_data: r.newData(bcopy)}
    return nil
}

func (r __node[D]) json() []byte {
    if r._data == nil {
        return r.defaultJson()
    }

    return r._data._json
}

func (r __node[D]) path() string {
    return r._path
}

func (r __node[D]) setJson(v []byte) {
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
        return gjson.GetBytes(parent.JSON(), r._ppath)
    }

    if r._path == "" {
        return gjson.ParseBytes(r.json())
    }
    return gjson.GetBytes(r.json(), r.path())
}

func (r __node[D]) Exists() bool {
    return r.result().Exists()
}

func (r __node[D]) Delete() error {
    res, err := sjson.DeleteBytes(r.json(), r.path())
    if err != nil {
        return err
    }
    r.setJson(res)
    return nil
}

func (r *__node[D]) set(incoming []byte) error {
    r.ensureJson()

    if r._path == "" {
        r.setJson(incoming)
        return nil
    }

    res, err := sjson.SetRawBytes(r.json(), r.path(), incoming)
    if err != nil {
        return err
    }
    r.setJson(res)
    return nil
}

func (r *__node[D]) setMerge(incoming []byte) error {
	current := r.currentJson()

	var buf bytes.Buffer
	buf.Grow(len(current)+len(incoming)+3)
	buf.WriteByte('[')
	buf.Write(current)
	buf.WriteByte(',')
	buf.Write(incoming)
	buf.WriteByte(']')

    incoming = []byte(gjson.GetBytes(buf.Bytes(), "@join").Raw)

    return r.set(incoming)
}

func (r __node[D]) copy() __node[D] {
    j := r.currentJson()
    b := make([]byte, len(j))
    copy(b, j)
    return __node[D]{
        _data: r.newData(b),
    }
}

func (r __node[D]) defaultJson() []byte {
    var d D
	return d.typeDefaultJson()
}

