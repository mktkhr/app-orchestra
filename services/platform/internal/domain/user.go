package domain

// Role identifies what an account may do beyond the permissions it holds
// individually (docs/specs/auth.md, A5). An admin may read every account
// and set anybody's permissions; that is the whole of what the role buys -
// an admin holds every permission implicitly, rather than through rows of
// its own (docs/specs/auth.md, section 4).
type Role string

// The two roles the platform knows (docs/specs/auth.md, A5).
const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

// User is a signed-in account (docs/specs/auth.md, section 3). Its
// password hash lives behind the Authenticator port, not here: this is
// what the rest of the platform is handed once a person is identified, and
// a hash is not something any of those callers need to see.
type User struct {
	ID   string
	Name string
	Role Role
}

// Permission says the user it was read for may call one operation of one
// service (docs/specs/auth.md, section 4, A3) - per operation rather than
// per service, because "may look at stock, may not create it" is a
// sentence this product has to be able to say and a service-level grant
// cannot say. There is no user id here: usecase.PermissionStore.For scopes
// the slice to one user already, the same way WorkspaceStore.List scopes
// its result to one owner.
type Permission struct {
	Service     string
	OperationID string
}
