package main

const (
	// 卡片类型
	CardTypeEmpty           = 0
	CardTypeAnniversary     = 1
	CardTypeAnniversaryList = 2
	CardTypeCountdown       = 3
	CardTypeCountdownList   = 4
	CardTypeProgress        = 5
	CardTypeToDoList        = 6
)

func cardContentCheck(card CardItemRequest) (ok bool, message string) {
	// 必需内容
	if card.Openid == "" {
		return false, "用户信息为空"
	}
	if card.Card.Type < 0 || card.Card.Type > 6 {
		return false, "卡片类型不合法"
	}
	if card.Card.Title == "" {
		return false, "卡片标题为空"
	}
	// 卡片类型内容检查
	switch card.Card.Type {
	case CardTypeEmpty:
		break
	case CardTypeAnniversary, CardTypeCountdown:
		if len(card.Card.Date) == 0 {
			return false, "日期为空"
		}
		if len(card.Card.Background) == 0 {
			return false, "背景图片为空"
		}
	case CardTypeAnniversaryList, CardTypeCountdownList:
		if len(card.Card.Background) == 0 {
			return false, "背景图片为空"
		}
		if len(card.Card.List) == 0 {
			return false, "列表为空"
		}
		for _, item := range card.Card.List {
			if item.Name == "" {
				return false, "列表项名称为空"
			}
			if item.Date == "" {
				return false, "列表项日期为空"
			}
			if item.Avatar == "" {
				return false, "列表项头像为空"
			}
		}
	}
	return true, ""
}
