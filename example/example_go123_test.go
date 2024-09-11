//go:build go1.23

//go:generate go run github.com/raphaelvigee/gensonschema/cmd
package gen_test

import (
	gen "github.com/raphaelvigee/gensonschema/example"
	"testing"
)

func TestGo123Range(t *testing.T) {
	var obj gen.ArrayArray

	for i, v := range obj.GetTopfield1().Range {
		// asserts
		var _ int = i
		var _ *gen.ArrayDefinitionsDef1 = v

		continue
	}
}
