// Copyright 2017 Eryx <evorui аt gmаil dοt cοm>, All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lessos/lessgo/types"
)

type pkg_types struct {
	pkgname string
	types   types.ArrayString
}

var (
	string_space_reg = regexp.MustCompile("\\ +")
	intypes          = types.ArrayString([]string{"string", "int", "int32", "int64", "uint", "uint32", "uint64", "bool", "float", "bytes"})
	buildins         = map[string]*pkg_types{}
)

func main() {

	proto_path := "*.proto"
	if len(os.Args) > 1 {
		proto_path = os.Args[1]
	}

	fmt.Println("try", proto_path)

	ls, err := filepath.Glob(proto_path)
	if err != nil {
		log.Fatal(err)
	}

	for _, vfile := range ls {
		if err := do_proto(vfile); err != nil {
			fmt.Println("ERROR", vfile, err.Error())
		}
	}

	for k, v := range buildins {
		filelib := k + "/pb.objs.go"
		fpl, err := os.OpenFile(filelib, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			log.Fatal(err)
		}
		fpl.Seek(0, 0)
		fpl.Truncate(0)

		fpl.WriteString(buildin_slice(v.pkgname, v.types))
		fpl.Close()

		fmt.Println("OK", filelib)
	}
}

type field_entry struct {
	field      string
	repeated   string
	equal_skip bool
	isObject   bool
	field_type string
}

type msg_entry struct {
	name   string
	key    types.KvPairs
	keyFmt string
	fields []field_entry
}

func do_proto(file string) error {

	fp, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fp.Close()

	fpbuf := bufio.NewReader(fp)

	var (
		list    []*msg_entry
		entry   *msg_entry
		pkgname = ""
		enums   types.ArrayString
		obj_ins types.ArrayString
	)

	for {

		bs, err := fpbuf.ReadBytes('\n')
		if err != nil {
			break
		}

		str := string_space_reg.ReplaceAllString(strings.TrimSpace(strings.Replace(string(bs), "\t", " ", -1)), " ")

		if strings.HasPrefix(str, "enum") {

			if entry != nil && entry.name != "" && len(entry.key) > 0 {
				list = append(list, entry)
			}
			entry = nil

			enums.Set(strings.TrimSpace(str[4 : len(str)-1]))
			continue
		} else if strings.HasPrefix(str, "message") {

			if entry != nil && entry.name != "" && len(entry.key) > 0 {
				list = append(list, entry)
			}

			entry = &msg_entry{
				name: strings.TrimSpace(str[7 : len(str)-1]),
			}
			continue
		} else if strings.HasPrefix(str, "package") {
			if i := strings.Index(str, ";"); i > 0 {
				pkgname = strings.TrimSpace(str[7:i])
			}
			continue
		}

		if entry == nil {
			continue
		}

		ps := strings.Split(str, " ")
		if len(ps) < 3 {
			continue
		}

		switch ps[0] {
		case "string", "int", "int32", "int64", "uint", "uint32", "uint64", "bool", "float":
			field := strings.Replace(strings.Title(strings.Replace(ps[1], "_", " ", -1)), " ", "", -1)
			equal_skip := false
			if strings.Contains(str, "struct:object_slice_equal_skip") {
				equal_skip = true
			}

			entry.fields = append(entry.fields, field_entry{
				field:      field,
				field_type: ps[0],
				equal_skip: equal_skip,
			})

			if strings.Contains(str, "struct:object_slice_key") {
				entry.key.Set(field, ps[0])
				if entry.keyFmt != "" {
					entry.keyFmt += "."
				}
				if ps[0] == "string" {
					entry.keyFmt += `%s`
				} else if ps[0] == "bool" {
					entry.keyFmt += `%b`
				} else {
					entry.keyFmt += `%d`
				}
			}

		case "bytes":
			obj_ins.Set("bytes")
			field := strings.Replace(strings.Title(strings.Replace(ps[1], "_", " ", -1)), " ", "", -1)
			entry.fields = append(entry.fields, field_entry{
				field:      field,
				field_type: "bytes",
			})

		case "repeated":
			if intypes.Has(ps[1]) {
				obj_ins.Set(ps[1])
				ps[1] = "Pb" + strings.Title(ps[1])
			}
			field := strings.Replace(strings.Title(strings.Replace(ps[2], "_", " ", -1)), " ", "", -1)
			field_rp := strings.Replace(ps[1], "_", " ", -1)
			entry.fields = append(entry.fields, field_entry{
				field:    field,
				repeated: field_rp,
			})

		default:
			if !strings.HasPrefix(ps[0], "/") {
				field := strings.Replace(strings.Title(strings.Replace(ps[1], "_", " ", -1)), " ", "", -1)
				isObject := false
				if !enums.Has(ps[0]) {
					isObject = true
				}
				entry.fields = append(entry.fields, field_entry{
					field:    field,
					isObject: isObject,
				})
			}
		}
	}

	if entry != nil && entry.name != "" && len(entry.key) > 0 {
		list = append(list, entry)
	}

	if pkgname == "" || len(list) == 0 {
		return nil
	}

	filename := file
	if i := strings.LastIndex(filename, "/"); i > 0 {
		filename = filename[i+1:]
	}

	code := "// Code generated by github.com/hooto/protobuf_slice\n"
	code += fmt.Sprintf("// source: %s\n// DO NOT EDIT!\n", filename)
	code += "\npackage " + pkgname + "\n\n"
	code += "import \"sync\"\n"
	/**
	code += "import \"fmt\"\n"
	code += "import \"github.com/lessos/lessgo/types\"\n"
	*/

	for _, v := range list {
		if v.name == "" || len(v.key) == 0 {
			continue
		}

		if len(v.fields) < 2 {
			continue
		}

		// locker
		code += fmt.Sprintf("\nvar object_slice_mu_%s sync.RWMutex\n", v.name)

		/**
		// IterKey
		code += fmt.Sprintf("\nfunc (it *%s) IterKey() string {\n", v.name)
		p_keys_pri := []string{}
		for _, pkv := range v.key {
			p_keys_pri = append(p_keys_pri, fmt.Sprintf("it.%s", pkv.Key))
		}
		code += fmt.Sprintf("\treturn fmt.Sprintf(\"%s\", %s)\n}\n", v.keyFmt, strings.Join(p_keys_pri, ", "))

		// IterEqual
		code += fmt.Sprintf("\nfunc (it *%s) IterEqual(it2 types.IterObject) bool {\n", v.name)
		code += fmt.Sprintf("\tif it2 == nil {\n\t\treturn false\n\t}\n")
		code += fmt.Sprintf("\n\tit3, ok := it2.(*%s)\n\tif !ok {\n\t\treturn false\n\t}\n", v.name)
		code_diff := ""
		for i, vf := range v.fields {

			if vf.equal_skip {
				continue
			}

			if code_diff == "" {
				code_diff += "\n\tif "
			} else {
				code_diff += " ||\n\t\t"
			}

			if vf.isObject {
				code_diff += fmt.Sprintf("(it.%s == nil && it3.%s != nil) || ", vf.field, vf.field)
				code_diff += fmt.Sprintf("(it.%s != nil && !it.%s.IterEqual(it3.%s))", vf.field, vf.field, vf.field)
			} else if vf.repeated == "" {
				code_diff += fmt.Sprintf("it.%s != it3.%s", vf.field, vf.field)
			} else {
				code_diff += fmt.Sprintf("!types.IterObjectsEqual(it.%s, it3.%s)", vf.field, vf.field)
			}

			if i+1 >= len(v.fields) {
				code_diff += " {\n\t\treturn false\n\t}\n"
			}
		}
		code += code_diff
		code += "\treturn true\n}\n"
		*/

		// equal
		code += fmt.Sprintf("\nfunc (it *%s) Equal(it2 *%s) bool {\n", v.name, v.name)
		code += fmt.Sprintf("\tif it2 == nil")
		for i, vf := range v.fields {

			if !vf.equal_skip {

				if vf.isObject {
					code += fmt.Sprintf(" ||\n\t\t(it.%s == nil && it2.%s != nil)", vf.field, vf.field)
					code += fmt.Sprintf(" ||\n\t\t(it.%s != nil && !it.%s.Equal(it2.%s))", vf.field, vf.field, vf.field)
				} else if vf.field_type == "bytes" {
					code += fmt.Sprintf(" ||\n\t\t!PbBytesSliceEqual(it.%s, it2.%s)", vf.field, vf.field)
				} else if vf.repeated == "" {
					code += fmt.Sprintf(" ||\n\t\tit.%s != it2.%s", vf.field, vf.field)
				} else {
					code += fmt.Sprintf(" ||\n\t\t!%sSliceEqual(it.%s, it2.%s)", vf.repeated, vf.field, vf.field)
				}
			}

			if i+1 >= len(v.fields) {
				code += " {\n\t\treturn false\n\t}\n"
			}
		}
		code += "\treturn true\n}\n"

		// sync
		code += fmt.Sprintf("\nfunc (it *%s) Sync(it2 *%s) bool {\n", v.name, v.name)
		code += fmt.Sprintf("\tif it2 == nil {\n\t\treturn false\n\t}\n")
		code += fmt.Sprintf("\tif it.Equal(it2) {\n\t\treturn false\n\t}\n")
		code += fmt.Sprintf("\t*it = *it2\n")
		code += fmt.Sprintf("\treturn true\n}\n")
		/**
		code += fmt.Sprintf("\tchanged := false\n")
		for _, vf := range v.fields {
			if vf.repeated == "" {
				code += fmt.Sprintf("\tif it.%s != it2.%s {\n\t\tit.%s, changed = it2.%s, true\n\t}\n",
					vf.field, vf.field, vf.field, vf.field)
			} else {
				code += fmt.Sprintf("\tif rs, ok := %sSliceSyncSlice(it.%s, it2.%s); ok {\n\t\tit.%s, changed = rs, true\n\t}\n",
					vf.repeated, vf.field, vf.field, vf.field)
			}
		}
		code += "\treturn changed\n}\n"
		*/

		// slice get
		p_keys := []string{}
		p_keys_eq := []string{}
		for _, pkv := range v.key {
			p_keys = append(p_keys, "arg_"+strings.ToLower(pkv.Key)+" "+pkv.Value)
			p_keys_eq = append(p_keys_eq, fmt.Sprintf("v.%s == arg_%s", pkv.Key, strings.ToLower(pkv.Key)))
		}
		code += fmt.Sprintf("\nfunc %sSliceGet(ls []*%s, %s) *%s {\n", v.name, v.name, strings.Join(p_keys, ", "), v.name)
		code += fmt.Sprintf("\tobject_slice_mu_%s.RLock()\n\tdefer object_slice_mu_%s.RUnlock()\n\n", v.name, v.name)
		code += fmt.Sprintf("\tfor _, v := range ls {\n")
		code += fmt.Sprintf("\t\tif %s {\n\t\t\treturn v\n\t\t}\n", strings.Join(p_keys_eq, " && "))
		code += "\t}\n"
		code += "\treturn nil\n}\n"

		// slice equal
		code += fmt.Sprintf("\nfunc %sSliceEqual(ls, ls2 []*%s) bool {\n", v.name, v.name)
		code += fmt.Sprintf("\tobject_slice_mu_%s.RLock()\n\tdefer object_slice_mu_%s.RUnlock()\n\n", v.name, v.name)
		code += fmt.Sprintf("\tif len(ls) != len(ls2) {\n\t\treturn false\n\t}\n")
		code += fmt.Sprintf("\thit := false\n")
		code += fmt.Sprintf("\tfor _, v := range ls {\n")
		code += fmt.Sprintf("\t\thit = false\n")
		code += fmt.Sprintf("\t\tfor _, v2 := range ls2 {\n")
		p_keys_eq = []string{}
		for _, pkv := range v.key {
			p_keys_eq = append(p_keys_eq, fmt.Sprintf("v.%s != v2.%s", pkv.Key, pkv.Key))
		}
		code += fmt.Sprintf("\t\t\tif %s {\n\t\t\t\tcontinue\n\t\t\t}\n", strings.Join(p_keys_eq, " || "))
		code += fmt.Sprintf("\t\t\tif !v.Equal(v2) {\n\t\t\t\treturn false\n\t\t\t}\n")
		/**
		for _, vf := range v.fields {
			if ok := v.key.Get(vf.field); ok != nil {
				continue
			}
			if vf.equal_skip {
				continue
			}
			if vf.repeated == "" {
				code += fmt.Sprintf("\t\t\tif v.%s != v2.%s {\n\t\t\t\treturn false\n\t\t\t}\n",
					vf.field, vf.field)
			} else {
				code += fmt.Sprintf("\t\t\tif !%sSliceEqual(v.%s, v2.%s) {\n\t\t\t\treturn false\n\t\t\t}\n",
					vf.repeated, vf.field, vf.field)
			}
		}
		*/
		code += "\t\t\thit = true\n\t\t\tbreak\n"
		code += "\t\t}\n"
		code += "\t\tif !hit {\n\t\t\treturn false\n\t\t}\n"
		code += "\t}\n"
		code += "\treturn true\n}\n"

		// slice sync
		code += fmt.Sprintf("\nfunc %sSliceSync(ls []*%s, it2 *%s) ([]*%s, bool) {\n", v.name, v.name, v.name, v.name)
		code += fmt.Sprintf("\tif it2 == nil {\n\t\treturn ls, false\n\t}\n")
		code += fmt.Sprintf("\tobject_slice_mu_%s.Lock()\n\tdefer object_slice_mu_%s.Unlock()\n\n", v.name, v.name)
		code += fmt.Sprintf("\thit := false\n")
		code += fmt.Sprintf("\tchanged := false\n")
		code += fmt.Sprintf("\tfor i, v := range ls {\n")
		p_keys_eq = []string{}
		for _, pkv := range v.key {
			p_keys_eq = append(p_keys_eq, fmt.Sprintf("v.%s != it2.%s", pkv.Key, pkv.Key))
		}
		code += fmt.Sprintf("\t\tif %s {\n\t\t\tcontinue\n\t\t}\n", strings.Join(p_keys_eq, " || "))
		code += fmt.Sprintf("\t\tif !v.Equal(it2) {\n\t\t\tls[i], changed = it2, true\n\t\t}\n")
		/**
		for _, vf := range v.fields {
			if ok := v.key.Get(vf.field); ok != nil {
				continue
			}
			if vf.repeated == "" {
				code += fmt.Sprintf("\t\tif v.%s != it2.%s {\n\t\t\tv.%s, changed = it2.%s, true\n\t\t}\n",
					vf.field, vf.field, vf.field, vf.field)
			} else {
				code += fmt.Sprintf("\t\tif rs, ok := %sSliceSyncSlice(v.%s, it2.%s); ok {\n\t\t\tv.%s, changed = rs, true\n\t\t}\n",
					vf.repeated, vf.field, vf.field, vf.field)
			}
		}
		*/
		code += "\t\thit = true\n\t\tbreak\n"
		code += "\t}\n"
		code += "\tif !hit {\n\t\tls = append(ls, it2)\n\t\tchanged = true\n\t}\n"
		code += "\treturn ls, changed\n}\n"

		// slice sync slice
		code += fmt.Sprintf("\nfunc %sSliceSyncSlice(ls, ls2 []*%s) ([]*%s, bool) {\n", v.name, v.name, v.name)
		code += fmt.Sprintf("\tif %sSliceEqual(ls, ls2) {\n\t\treturn ls, false\n\t}\n", v.name)
		code += fmt.Sprintf("\treturn ls2, true\n}\n")
		/**
		code += fmt.Sprintf("\nfunc %sSliceSyncSlice(ls, ls2 []*%s) ([]*%s, bool) {\n", v.name, v.name, v.name)
		code += fmt.Sprintf("\tif len(ls2) == 0 {\n\t\treturn ls, false\n\t}\n")
		code += fmt.Sprintf("\tobject_slice_mu_%s.Lock()\n\tdefer object_slice_mu_%s.Unlock()\n\n", v.name, v.name)
		code += fmt.Sprintf("\thit := false\n")
		code += fmt.Sprintf("\tchanged := false\n")
		code += fmt.Sprintf("\tfor _, v2 := range ls2 {\n")
		code += fmt.Sprintf("\t\thit = false\n")
		code += fmt.Sprintf("\t\tfor _, v := range ls {\n")
		p_keys_eq = []string{}
		for _, pkv := range v.key {
			p_keys_eq = append(p_keys_eq, fmt.Sprintf("v.%s != v2.%s", pkv.Key, pkv.Key))
		}
		code += fmt.Sprintf("\t\t\tif %s {\n\t\t\t\tcontinue\n\t\t\t}\n", strings.Join(p_keys_eq, " || "))
		for _, vf := range v.fields {
			if ok := v.key.Get(vf.field); ok != nil {
				continue
			}
			if vf.repeated == "" {
				code += fmt.Sprintf("\t\t\tif v.%s != v2.%s {\n\t\t\t\tv.%s, changed = v2.%s, true\n\t\t\t}\n",
					vf.field, vf.field, vf.field, vf.field)
			} else {
				code += fmt.Sprintf("\t\t\tif rs, ok := %sSliceSyncSlice(v.%s, v2.%s); ok {\n\t\t\t\tv.%s, changed = rs, true\n\t\t\t}\n",
					vf.repeated, vf.field, vf.field, vf.field)
			}
		}
		code += "\t\t\thit = true\n\t\t\tbreak\n"
		code += "\t\t}\n"
		code += "\t\tif !hit {\n\t\t\tls = append(ls, v2)\n\t\t\tchanged = true\n\t\t}\n"
		code += "\t}\n"
		code += "\treturn ls, changed\n}\n"
		*/
	}

	{
		fileout := file
		if strings.HasSuffix(fileout, ".proto") {
			fileout = fileout[:len(fileout)-6] + ".pb"
		}
		fileout += ".objs.go"

		fpo, err := os.OpenFile(fileout, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return err
		}
		fpo.Seek(0, 0)
		fpo.Truncate(0)

		fpo.WriteString(code)
		fpo.Close()
		fmt.Println("OK", fileout)
	}

	{
		ins, _ := buildins[filepath.Dir(file)]
		if ins == nil {
			ins = &pkg_types{
				pkgname: pkgname,
			}
		}
		for _, v := range obj_ins {
			ins.types.Set(v)
		}
		buildins[filepath.Dir(file)] = ins
	}

	return nil
}

func buildin_slice(pkgname string, ts types.ArrayString) string {

	code := "// Code generated by github.com/hooto/protobuf_slice\n"
	code += "// DO NOT EDIT!\n\npackage " + pkgname

	bytesDiff := false

	for _, v := range intypes {
		if v == "bytes" {
			bytesDiff = true
			break
		}
	}

	if bytesDiff {
		code += `

import "bytes"

func PbBytesSliceEqual(ls, ls2 []byte) bool {
	if len(ls) != len(ls2) {
		return false
	}
	return bytes.Compare(ls, ls2) == 0
}`
	}

	for _, v := range intypes {

		if !ts.Has(v) {
			continue
		}

		if v == "bytes" {
			continue
		}

		code += fmt.Sprintf(`

func Pb%sSliceEqual(ls, ls2 []%s) bool {
	if len(ls) != len(ls2) {
		return false
	}
	for _, v := range ls {
		hit := false
		for _, v2 := range ls2 {
			if v == v2 {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}`, strings.Title(v), v)

		code += fmt.Sprintf(`

func Pb%sSliceSyncSlice(ls, ls2 []%s) ([]%s, bool) {
	if len(ls2) == 0 {
		return ls, false
	}
	hit := false
	changed := false
	for _, v2 := range ls2 {
		hit = false
		for _, v := range ls {
			if v == v2 {
				hit = true
				break
			}
		}
		if !hit {
			ls = append(ls, v2)
			changed = true
		}
	}
	return ls, changed
}`, strings.Title(v), v, v)

	}

	return code
}
