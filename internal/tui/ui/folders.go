// Package ui holds shared bubble tea/lipgloss primitives (theme, sidebar,
// folder definitions, key bindings) used across TUI pages.
package ui

// DefaultFolders defines the standard folder hierarchy matching the silo service defaults.
// Children inherit the parent's schema for filtering purposes.
var DefaultFolders = []FolderDef{
	{
		Slug:   "invoices",
		Name:   "Invoices",
		Schema: "bill/invoice",
		Children: []FolderDef{
			{Slug: "sales", Name: "Sales"},
			{Slug: "expenses", Name: "Expenses"},
		},
	},
	{
		Slug:   "orders",
		Name:   "Orders",
		Schema: "bill/order",
	},
	{
		Slug:   "deliveries",
		Name:   "Deliveries",
		Schema: "bill/delivery",
	},
	{
		Slug:   "payments",
		Name:   "Payments",
		Schema: "bill/payment",
	},
	{
		Slug:   "contacts",
		Name:   "Parties",
		Schema: "org/party",
		Children: []FolderDef{
			{Slug: "suppliers", Name: "Suppliers"},
			{Slug: "customers", Name: "Customers"},
		},
	},
	{
		Slug:   "items",
		Name:   "Items",
		Schema: "org/item",
	},
}

// FolderDef describes a folder in the hierarchy.
type FolderDef struct {
	Slug     string
	Name     string
	Schema   string // only set on parent; children inherit
	Children []FolderDef
}
