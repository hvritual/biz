package application

import (
	"context"
	"strings"
	deviceopsv1 "github.com/hvritual/biz/contracts/gen/deviceops/v1"
	"github.com/hvritual/biz/internal/deviceops/domain"
	"github.com/hvritual/biz/internal/deviceops/ports"
	"yunka.io/framework/requestscope"
)

func (s *SiteManagementService) ListAssignableRoleSites(ctx context.Context,r *deviceopsv1.SiteScopeDirectoryRequest)(*deviceopsv1.SiteScopeDirectoryResponse,error){if r==nil||!r.GetResolve()||len(r.GetSiteIds())==0{return nil,ErrInvalid};seen:=map[string]struct{}{};ids:=make([]string,0,len(r.GetSiteIds()));for _,raw:=range r.GetSiteIds(){id:=strings.TrimSpace(raw);if id==""{return nil,ErrInvalid};if _,ok:=seen[id];ok{continue};seen[id]=struct{}{};ids=append(ids,id)};sites,err:=requestscope.JoinValue(ctx,s.repositories,func(v *requestscope.View[ports.ScopedRepositories])([]domain.Site,error){out:=make([]domain.Site,0,len(ids));for _,id:=range ids{site,err:=v.Repositories().Site.Get(v.Context(),id);if err!=nil{return nil,err};out=append(out,site)};return out,nil});if err!=nil{return nil,err};response:=&deviceopsv1.SiteScopeDirectoryResponse{Sites:make([]*deviceopsv1.SiteDTO,0,len(sites)),Total:uint64(len(sites))};for _,site:=range sites{response.Sites=append(response.Sites,&deviceopsv1.SiteDTO{Id:site.ID,Name:site.Name,Version:site.Version})};return response,nil}
