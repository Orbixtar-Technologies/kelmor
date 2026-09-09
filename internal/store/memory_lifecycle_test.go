package store

import "testing"

func TestDeletePackageIfUnusedIsAtomicWithAccountAssignment(t *testing.T) {
	data := NewMemory()
	assigned := &Package{ID: "assigned", Name: "Assigned"}
	unassigned := &Package{ID: "unassigned", Name: "Unassigned"}
	data.PutPackage(assigned)
	data.PutPackage(unassigned)
	data.PutAccount(&Account{ID: "account", PackageID: assigned.ID})

	if data.DeletePackageIfUnused(assigned.ID) {
		t.Fatal("assigned package was deleted")
	}
	if data.GetPackage(assigned.ID) == nil {
		t.Fatal("assigned package is missing")
	}
	if !data.DeletePackageIfUnused(unassigned.ID) {
		t.Fatal("unassigned package was not deleted")
	}
	if data.GetPackage(unassigned.ID) != nil {
		t.Fatal("unassigned package remains")
	}
}
