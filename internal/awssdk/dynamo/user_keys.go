package dynamo

import "fmt"

// User keys
func UserPK(userId string) string { return fmt.Sprintf("USER#%s", userId) }
func UserSK(userId string) string { return fmt.Sprintf("USER#%s", userId) }

// UserPrimaryKey returns a full PK/SK pair for a user id.
func UserPrimaryKey(userId string) Item {
    return Item{
        "PK": StringAttribute(UserPK(userId)),
        "SK": StringAttribute(UserSK(userId)),
    }
}

// Uniqueness guards (internal)
func UserEmailPK(email string) string            { return fmt.Sprintf("USER_EMAIL#%s", email) }
func UserPhonePK(phone string) string            { return fmt.Sprintf("USER_PHONE#%s", phone) }
func UserPreferredUsernamePK(u string) string    { return fmt.Sprintf("USER_PREFERREDUSERNAME#%s", u) }
