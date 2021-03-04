# 自动刷新率
 
## 构建
```
CC=$NDK_HOMT/toolchains/llvm/prebuilt/darwin-x86_64/bin/armv7a-linux-androideabi30-clang CXX=$NDK_HOMT/toolchains/llvm/prebuilt/darwin-x86_64/bin/armv7a-linux-androideabi30-clang GOOS=linux GOARCH=arm CGO_ENABLED=0 go build
```

## 配置文件`/sdcard/afps_nzlov.conf`
```
package idlefps touchingfps
package/activity idlefps touchingfps
* idlefps touchingfps
```

## 注意
* 配置自动生效，不需要重启
* 停止触摸1s后更新为`idlefps`
* GSI类取消强制FPS，官方系统设置为最低刷新率
* 与其他设置刷新率冲突
