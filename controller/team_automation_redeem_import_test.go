package controller

import "testing"

func TestParseRedeemCodeImportItemsFromText(t *testing.T) {
	raw := `
# comment
foo@example.com,TEAM-AAA,helloteam.store
12,TEAM-BBB,teamxz.store
13,bar@example.com,TEAM-CCC,https://helloteam.store/redeem
`
	items := parseRedeemCodeImportItemsFromText(raw)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].Email != "foo@example.com" || items[0].RedeemCode != "TEAM-AAA" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].ID != 12 || items[1].RedeemCode != "TEAM-BBB" {
		t.Fatalf("unexpected second item: %#v", items[1])
	}
	if items[2].ID != 13 || items[2].Email != "bar@example.com" || items[2].RedeemSite != "helloteam.store/redeem" {
		t.Fatalf("unexpected third item: %#v", items[2])
	}
}
