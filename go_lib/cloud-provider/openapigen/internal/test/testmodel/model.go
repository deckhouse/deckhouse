package testmodel

// +deckhouse:ru:description=Объявление товара
// +deckhouse:ru:description=
// +deckhouse:ru:description=Каждый товар может быть связан с несколькими пользователями
type GoodRef struct {
	// +deckhouse:description=The good's ID
	// +deckhouse:default=good-deadbeefcafe
	// +required
	ID string `json:"id,omitempty"`
	// +deckhouse:description=The good's name
	// +required
	Name string `json:"name,omitempty"`
	// +deckhouse:description=The good's price
	// +required
	Price int `json:"price,omitempty"`
}
