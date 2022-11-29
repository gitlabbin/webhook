package job

import (
	"github.com/oliveagle/jsonpath"
)

func Get(s string, params interface{}) (interface{}, error) {

	res, err := jsonpath.JsonPathLookup(params, "$.expensive")

	//or reuse lookup pattern
	pat, _ := jsonpath.Compile(`$.store.book[?(@.price < $.expensive)].price`)
	res, err = pat.Lookup(params)
	return res, err
}
