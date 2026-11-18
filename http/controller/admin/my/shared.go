package my

import (
	"github.com/gin-gonic/gin"
	"github.com/lejianwen/rustdesk-api/v2/http/request/admin"
	"github.com/lejianwen/rustdesk-api/v2/http/response"
	"github.com/lejianwen/rustdesk-api/v2/model"
	"github.com/lejianwen/rustdesk-api/v2/service"
	"gorm.io/gorm"
)

// Shared mostra no painel, só para leitura, os livros e contatos que outros
// usuários compartilharam com o usuário logado (regra pessoal ou do grupo dele).
// É a mesma regra que o app usa em /api/ab/shared/profiles. Editar continua
// pelo app, que respeita leitura / leitura e escrita / controle total.
type Shared struct {
}

type sharedCollection struct {
	Id    uint   `json:"id"`
	Name  string `json:"name"`
	Owner string `json:"owner"`
	Rule  int    `json:"rule"`
}

// livros compartilhados com o usuário (sem os dele), com a maior permissão de cada um
func sharedCollections(user *model.User) []*sharedCollection {
	maxRule := map[uint]int{}
	for _, r := range service.AllService.AddressBookService.CollectionReadRules(user) {
		if r.Rule > maxRule[r.CollectionId] {
			maxRule[r.CollectionId] = r.Rule
		}
	}
	if len(maxRule) == 0 {
		return []*sharedCollection{}
	}
	ids := make([]uint, 0, len(maxRule))
	for id := range maxRule {
		ids = append(ids, id)
	}
	collections := service.AllService.AddressBookService.ListCollectionByIds(ids)
	ownerIds := make([]uint, 0, len(collections))
	for _, c := range collections {
		ownerIds = append(ownerIds, c.UserId)
	}
	owners := map[uint]string{}
	for _, u := range service.AllService.UserService.ListByIds(ownerIds) {
		owners[u.Id] = u.Username
	}
	res := make([]*sharedCollection, 0, len(collections))
	for _, c := range collections {
		if c.UserId == user.Id {
			continue
		}
		owner, ok := owners[c.UserId]
		if !ok {
			continue // dono excluído
		}
		res = append(res, &sharedCollection{Id: c.Id, Name: c.Name, Owner: owner, Rule: maxRule[c.Id]})
	}
	return res
}

// Collections lista os livros compartilhados com o usuário
func (s *Shared) Collections(c *gin.Context) {
	u := service.AllService.UserService.CurUser(c)
	response.Success(c, sharedCollections(u))
}

// AddressBooks lista os contatos dos livros compartilhados (sem senha nem hash)
func (s *Shared) AddressBooks(c *gin.Context) {
	query := &admin.AddressBookQuery{}
	if err := c.ShouldBindQuery(query); err != nil {
		response.Fail(c, 101, response.TranslateMsg(c, "ParamsError")+err.Error())
		return
	}
	u := service.AllService.UserService.CurUser(c)
	cols := sharedCollections(u)
	permitidos := make([]uint, 0, len(cols))
	for _, col := range cols {
		if query.CollectionId == nil || *query.CollectionId <= 0 || uint(*query.CollectionId) == col.Id {
			permitidos = append(permitidos, col.Id)
		}
	}
	if len(permitidos) == 0 {
		response.Success(c, &model.AddressBookList{AddressBooks: []*model.AddressBook{}})
		return
	}
	res := service.AllService.AddressBookService.List(query.Page, query.PageSize, func(tx *gorm.DB) {
		tx.Preload("Collection", func(txc *gorm.DB) *gorm.DB {
			return txc.Select("id,name")
		})
		tx.Where("collection_id in ?", permitidos)
		if query.Id != "" {
			tx.Where("id like ?", "%"+query.Id+"%")
		}
		if query.Hostname != "" {
			tx.Where("hostname like ?", "%"+query.Hostname+"%")
		}
		if query.Username != "" {
			tx.Where("alias like ?", "%"+query.Username+"%")
		}
	})
	for _, ab := range res.AddressBooks {
		ab.Password = ""
		ab.Hash = ""
	}
	response.Success(c, res)
}
