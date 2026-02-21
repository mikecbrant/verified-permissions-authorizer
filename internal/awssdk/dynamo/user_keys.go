package dynamo

import "fmt"

// UserPK returns the partition key for a user record.
func UserPK(userId string) string { return fmt.Sprintf("USER#%s", userId) }

// UserSK returns the sort key for a user record.
func UserSK(userId string) string { return fmt.Sprintf("USER#%s", userId) }

// UserPrimaryKey returns a full PK/SK pair for a user id.
func UserPrimaryKey(userId string) Item {
	return Item{
		"PK": StringAttribute(UserPK(userId)),
		"SK": StringAttribute(UserSK(userId)),
	}
}

// UserEmailPK returns the partition key for a user email uniqueness guard row.
func UserEmailPK(email string) string { return fmt.Sprintf("USER_EMAIL#%s", email) }

// UserPhonePK returns the partition key for a user phone uniqueness guard row.
func UserPhonePK(phone string) string { return fmt.Sprintf("USER_PHONE#%s", phone) }

// UserPreferredUsernamePK returns the partition key for a preferred username uniqueness guard row.
func UserPreferredUsernamePK(u string) string { return fmt.Sprintf("USER_PREFERREDUSERNAME#%s", u) }
