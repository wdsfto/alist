package _23

import (
	"fmt"
	"github.com/Xhofe/alist/conf"
	"github.com/Xhofe/alist/drivers/base"
	"github.com/Xhofe/alist/model"
	"github.com/Xhofe/alist/utils"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	url "net/url"
	"path/filepath"
)

type Pan123 struct{}

func (driver Pan123) Config() base.DriverConfig {
	return base.DriverConfig{
		Name:      "123Pan",
		OnlyProxy: false,
	}
}

func (driver Pan123) Items() []base.Item {
	return []base.Item{
		{
			Name:        "username",
			Label:       "username",
			Type:        base.TypeString,
			Required:    true,
			Description: "account username/phone number",
		},
		{
			Name:        "password",
			Label:       "password",
			Type:        base.TypeString,
			Required:    true,
			Description: "account password",
		},
		{
			Name:     "root_folder",
			Label:    "root folder file_id",
			Type:     base.TypeString,
			Required: false,
		},
		{
			Name:     "order_by",
			Label:    "order_by",
			Type:     base.TypeSelect,
			Values:   "name,fileId,updateAt,createAt",
			Required: true,
		},
		{
			Name:     "order_direction",
			Label:    "order_direction",
			Type:     base.TypeSelect,
			Values:   "asc,desc",
			Required: true,
		},
	}
}

func (driver Pan123) Save(account *model.Account, old *model.Account) error {
	if account.RootFolder == "" {
		account.RootFolder = "0"
	}
	err := driver.Login(account)
	return err
}

func (driver Pan123) File(path string, account *model.Account) (*model.File, error) {
	path = utils.ParsePath(path)
	if path == "/" {
		return &model.File{
			Id:        account.RootFolder,
			Name:      account.Name,
			Size:      0,
			Type:      conf.FOLDER,
			Driver:    driver.Config().Name,
			UpdatedAt: account.UpdatedAt,
		}, nil
	}
	dir, name := filepath.Split(path)
	files, err := driver.Files(dir, account)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.Name == name {
			return &file, nil
		}
	}
	return nil, base.ErrPathNotFound
}

func (driver Pan123) Files(path string, account *model.Account) ([]model.File, error) {
	path = utils.ParsePath(path)
	var rawFiles []Pan123File
	cache, err := base.GetCache(path, account)
	if err == nil {
		rawFiles, _ = cache.([]Pan123File)
	} else {
		file, err := driver.File(path, account)
		if err != nil {
			return nil, err
		}
		rawFiles, err = driver.GetFiles(file.Id, account)
		if err != nil {
			return nil, err
		}
		if len(rawFiles) > 0 {
			_ = base.SetCache(path, rawFiles, account)
		}
	}
	files := make([]model.File, 0)
	for _, file := range rawFiles {
		files = append(files, *driver.FormatFile(&file))
	}
	return files, nil
}

func (driver Pan123) Link(path string, account *model.Account) (*base.Link, error) {
	file, err := driver.GetFile(utils.ParsePath(path), account)
	if err != nil {
		return nil, err
	}
	var resp Pan123DownResp
	_, err = pan123Client.R().SetResult(&resp).SetHeader("authorization", "Bearer "+account.AccessToken).
		SetBody(base.Json{
			"driveId":   0,
			"etag":      file.Etag,
			"fileId":    file.FileId,
			"fileName":  file.FileName,
			"s3keyFlag": file.S3KeyFlag,
			"size":      file.Size,
			"type":      file.Type,
		}).Post("https://www.123pan.com/api/file/download_info")
	if err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		if resp.Code == 401 {
			err := driver.Login(account)
			if err != nil {
				return nil, err
			}
			return driver.Link(path, account)
		}
		return nil, fmt.Errorf(resp.Message)
	}
	u, err := url.Parse(resp.Data.DownloadUrl)
	if err != nil {
		return nil, err
	}
	u_ := fmt.Sprintf("https://%s%s", u.Host, u.Path)
	res, err := base.NoRedirectClient.R().SetQueryParamsFromValues(u.Query()).Get(u_)
	if err != nil {
		return nil, err
	}
	log.Debug(res.String())
	link := base.Link{}
	if res.StatusCode() == 302 {
		link.Url = res.Header().Get("location")
	}else {
		link.Url = resp.Data.DownloadUrl
	}
	return &link, nil
}

func (driver Pan123) Path(path string, account *model.Account) (*model.File, []model.File, error) {
	path = utils.ParsePath(path)
	log.Debugf("pan123 path: %s", path)
	file, err := driver.File(path, account)
	if err != nil {
		return nil, nil, err
	}
	if !file.IsDir() {
		link, err := driver.Link(path, account)
		if err != nil {
			return nil, nil, err
		}
		file.Url = link.Url
		return file, nil, nil
	}
	files, err := driver.Files(path, account)
	if err != nil {
		return nil, nil, err
	}
	return nil, files, nil
}

func (driver Pan123) Proxy(c *gin.Context, account *model.Account) {
	c.Request.Header.Del("origin")
}

func (driver Pan123) Preview(path string, account *model.Account) (interface{}, error) {
	return nil, base.ErrNotSupport
}

func (driver Pan123) MakeDir(path string, account *model.Account) error {
	return base.ErrNotImplement
}

func (driver Pan123) Move(src string, dst string, account *model.Account) error {
	return base.ErrNotImplement
}

func (driver Pan123) Copy(src string, dst string, account *model.Account) error {
	return base.ErrNotImplement
}

func (driver Pan123) Delete(path string, account *model.Account) error {
	return base.ErrNotImplement
}

func (driver Pan123) Upload(file *model.FileStream, account *model.Account) error {
	return base.ErrNotImplement
}

var _ base.Driver = (*Pan123)(nil)
