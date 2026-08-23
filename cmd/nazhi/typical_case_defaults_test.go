package main

import "testing"

func TestTypicalCaseList_DefaultPageSizeMatchesFrontend(t *testing.T) {
	flag := typicalCaseListCmd.Flags().Lookup("page-size")
	if flag == nil {
		t.Fatal("典型案例列表应注册 page-size 参数")
	}
	if flag.DefValue != "10" {
		t.Fatalf("前端默认 pageSize=10，CLI 实际默认 %s", flag.DefValue)
	}
}
