// @name: 应用版本服务
// @Descripttion: 提供当前应用发布版本
// @version: 1.0.0
// @Author: sm
// @Date: 2026-09-07 12:43:38
// @LastEditTime: 2026-09-07 12:43:38
// @FilePath: version_service.go
package main

const AppVersion = "v2.11.15"

type VersionService struct {
	version string
}

func NewVersionService() *VersionService {
	return &VersionService{version: AppVersion}
}

func (vs *VersionService) CurrentVersion() string {
	return vs.version
}
