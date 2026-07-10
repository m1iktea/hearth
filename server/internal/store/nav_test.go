package store

import (
	"path/filepath"
	"testing"
)

func newTestNav(t *testing.T) *NavStore {
	t.Helper()
	n, err := OpenNav(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("OpenNav: %v", err)
	}
	t.Cleanup(func() { n.Close() })
	return n
}

func TestCategoryCRUD(t *testing.T) {
	n := newTestNav(t)

	cat, err := n.CreateCategory("基础设施", 1)
	if err != nil || cat.ID == 0 || cat.Name != "基础设施" {
		t.Fatalf("CreateCategory: %+v, %v", cat, err)
	}

	cat2, err := n.UpdateCategory(cat.ID, "服务", 2)
	if err != nil || cat2.Name != "服务" || cat2.SortOrder != 2 {
		t.Fatalf("UpdateCategory: %+v, %v", cat2, err)
	}

	if err := n.DeleteCategory(cat.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	cats, _ := n.ListCategories()
	if len(cats) != 0 {
		t.Errorf("want empty, got %+v", cats)
	}
}

func TestItemCRUDAndListNesting(t *testing.T) {
	n := newTestNav(t)
	cat, _ := n.CreateCategory("服务", 1)

	item, err := n.CreateItem(Item{CategoryID: cat.ID, Name: "PVE", URL: "https://pve:8006", Icon: "server", SortOrder: 1})
	if err != nil || item.ID == 0 {
		t.Fatalf("CreateItem: %+v, %v", item, err)
	}

	item.Name = "Proxmox"
	updated, err := n.UpdateItem(item)
	if err != nil || updated.Name != "Proxmox" {
		t.Fatalf("UpdateItem: %+v, %v", updated, err)
	}

	cats, err := n.ListCategories()
	if err != nil || len(cats) != 1 || len(cats[0].Items) != 1 {
		t.Fatalf("ListCategories: %+v, %v", cats, err)
	}
	if cats[0].Items[0].Name != "Proxmox" {
		t.Errorf("item = %+v", cats[0].Items[0])
	}

	if err := n.DeleteItem(item.ID); err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
}

func TestDeleteCategoryCascadesItems(t *testing.T) {
	n := newTestNav(t)
	cat, _ := n.CreateCategory("临时", 1)
	n.CreateItem(Item{CategoryID: cat.ID, Name: "x", URL: "http://x"})

	if err := n.DeleteCategory(cat.ID); err != nil {
		t.Fatalf("DeleteCategory: %v", err)
	}
	cats, _ := n.ListCategories()
	if len(cats) != 0 {
		t.Errorf("want cascade delete, got %+v", cats)
	}
}
