package inventory

import "context"

// Repository is the outbound port used by the inventory application service.
type Repository interface {
	List(context.Context) ([]Item, error)
	Get(context.Context, string) (Item, error)
	Create(context.Context, Item) (Item, error)
	Update(context.Context, string, Item) (Item, error)
	Delete(context.Context, string) error
}
