package tpl

func pathJoin(p1, p2 string) string {
    if p1 == "" {
        return p2
    }

    return p1+"."+p2
}

type __delegate interface {
    typeDefaultJson() []byte
}

type __node[D __delegate] struct {
	_json *[]byte
	_path string
}

func (r __node[D]) currentJson() []byte {
    if r._path == "" {
        return r.json()
    }

    res := r.result()
    return []byte(res.Raw)
}

func (r __node[D]) MarshalJSON() ([]byte, error) {
    return r.currentJson(), nil
}

func (r __node[D]) JSON() []byte {
    return r.currentJson()
}

func (r *__node[D]) UnmarshalJSON(b []byte) error {
    if r._json != nil {
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

    *r = __node[D]{_json: &bcopy}
    return nil
}

func (r __node[D]) json() []byte {
    if r._json == nil {
        return r.defaultJson()
    }

    return *r._json
}

func (r __node[D]) path() string {
    return r._path
}

func (r __node[D]) setJson(v []byte) {
    *r._json = v
}

func (r *__node[D]) ensureJson() {
    if r._json != nil {
        return
    }

    b := r.json()
    r._json = &b
}

func (r __node[D]) result() gjson.Result {
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

func (r *__node[D]) setArray(incoming []byte) error {
    param := []byte{'['}
    param = append(param, r.currentJson()...)
    param = append(param, ',')
    param = append(param, incoming...)
    param = append(param, ']')

    incoming = []byte(gjson.GetBytes(param, "@join").Raw)

    return r.set(incoming)
}

func (r __node[D]) copy() __node[D] {
    j := r.currentJson()
    b := make([]byte, len(j))
    copy(b, j)
    return __node[D]{
        _json: &b,
    }
}

func (r __node[D]) defaultJson() []byte {
    var d D
	return d.typeDefaultJson()
}

