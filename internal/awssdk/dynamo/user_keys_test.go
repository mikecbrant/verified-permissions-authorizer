package dynamo

import "testing"

func TestUserKeys(t *testing.T) {
	if UserPK("u") != "USER#u" {
		t.Fatalf("UserPK")
	}
	if UserSK("u") != "USER#u" {
		t.Fatalf("UserSK")
	}
	if UserEmailPK("e") != "USER_EMAIL#e" {
		t.Fatalf("UserEmailPK")
	}
	if UserPhonePK("p") != "USER_PHONE#p" {
		t.Fatalf("UserPhonePK")
	}
	if UserPreferredUsernamePK("u") != "USER_PREFERREDUSERNAME#u" {
		t.Fatalf("UserPreferredUsernamePK")
	}
}
