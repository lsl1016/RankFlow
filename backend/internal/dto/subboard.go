package dto

import "rankflow/internal/service"

type ResolveSubBoardRequest struct {
	Timestamp  int64             `json:"timestamp"`
	Dimensions map[string]string `json:"dimensions"`
}

type SetSubBoardStatusRequest struct {
	TypeID string `json:"typeId" binding:"required"`
	Status int    `json:"status" binding:"oneof=1 2"`
}

type SubBoardDTO struct {
	RankID      int64             `json:"rankId"`
	TypeID      string            `json:"typeId"`
	Dimensions  map[string]string `json:"dimensions"`
	Status      int               `json:"status"`
	MemberCount int64             `json:"memberCount"`
}

type SubBoardListData struct {
	List []SubBoardDTO `json:"list"`
}

func FromSubBoard(sb *service.SubBoard) SubBoardDTO {
	return SubBoardDTO{
		RankID:      sb.RankID,
		TypeID:      sb.TypeID,
		Dimensions:  sb.Dimensions,
		Status:      sb.Status,
		MemberCount: sb.MemberCount,
	}
}

func FromSubBoards(rows []service.SubBoard) SubBoardListData {
	list := make([]SubBoardDTO, 0, len(rows))
	for i := range rows {
		list = append(list, FromSubBoard(&rows[i]))
	}
	return SubBoardListData{List: list}
}
