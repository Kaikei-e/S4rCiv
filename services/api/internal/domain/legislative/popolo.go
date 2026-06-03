package legislative

import (
	"slices"
	"sort"
	"strings"
)

// Popolo entities projected from a meeting's speeches. Pure and deterministic:
// the same speeches always yield the same entities in the same order, so the
// projection is reproject-safe.

const ClassificationParliamentaryGroup = "parliamentary_group" // 会派

type Person struct {
	PersonID           string
	Name               string
	Yomi               string
	IdentityConfidence string
}

type Organization struct {
	OrgID          string
	Name           string
	Classification string
}

type Membership struct {
	PersonID string
	OrgID    string
	Role     string
}

// Entities bundles the projected Popolo entities plus a normalized-name index
// used to (conservatively) resolve vote rosters within the same meeting.
type Entities struct {
	Persons       []Person
	Organizations []Organization
	Memberships   []Membership
	NameToPerson  map[string]string
}

// BuildEntities derives people / 会派 / memberships from a meeting's speeches.
// Pseudo-speakers (会議録情報) and procedural roles (議長) are not people.
func BuildEntities(speeches []Speech) Entities {
	persons := map[string]Person{}
	orgs := map[string]Organization{}
	mems := map[string]Membership{}
	nameToPerson := map[string]string{}

	for _, s := range speeches {
		if IsNonPerson(s.Speaker) {
			continue
		}
		pid, conf := PersonIdentity(s.Speaker, s.SpeakerYomi)
		if _, ok := persons[pid]; !ok {
			persons[pid] = Person{
				PersonID:           pid,
				Name:               strings.TrimSpace(s.Speaker),
				Yomi:               strings.TrimSpace(s.SpeakerYomi),
				IdentityConfidence: conf,
			}
		}
		nameToPerson[normalizeName(s.Speaker)] = pid

		grp := strings.TrimSpace(s.SpeakerGroup)
		if grp == "" {
			continue
		}
		oid := OrganizationIdentity(grp)
		orgs[oid] = Organization{OrgID: oid, Name: grp, Classification: ClassificationParliamentaryGroup}
		mkey := pid + "|" + oid
		if _, ok := mems[mkey]; !ok {
			mems[mkey] = Membership{PersonID: pid, OrgID: oid, Role: strings.TrimSpace(s.SpeakerPosition)}
		}
	}

	return Entities{
		Persons:       sortedPersons(persons),
		Organizations: sortedOrgs(orgs),
		Memberships:   sortedMemberships(mems),
		NameToPerson:  nameToPerson,
	}
}

// ResolveVoter returns the person_id for a raw voter name when it unambiguously
// matches a person seen in the same meeting; empty otherwise (ADR-000004:
// link only when unambiguous, never guess).
func (e Entities) ResolveVoter(name string) string {
	return e.NameToPerson[normalizeName(name)]
}

func IsNonPerson(name string) bool {
	n := normalizeName(name)
	if n == "" || isRoleLabel(n) {
		return true
	}
	return slices.Contains([]string{"会議録情報", "議事日程", "本日の会議に付した案件"}, n)
}

func sortedPersons(m map[string]Person) []Person {
	out := make([]Person, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PersonID < out[j].PersonID })
	return out
}

func sortedOrgs(m map[string]Organization) []Organization {
	out := make([]Organization, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OrgID < out[j].OrgID })
	return out
}

func sortedMemberships(m map[string]Membership) []Membership {
	out := make([]Membership, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].PersonID != out[j].PersonID {
			return out[i].PersonID < out[j].PersonID
		}
		return out[i].OrgID < out[j].OrgID
	})
	return out
}
