package main

import "testing"

func TestHonorList_DefaultPageSizeMatchesFrontend(t *testing.T) {
	flag := honorListCmd.Flags().Lookup("page-size")
	if flag == nil {
		t.Fatal("荣誉列表应注册 page-size 参数")
	}
	if flag.DefValue != "10" {
		t.Fatalf("前端默认 pageSize=10，CLI 实际默认 %s", flag.DefValue)
	}
}
