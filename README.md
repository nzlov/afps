# 自动刷新率
 
## 构建
```
CC=$NDK_HOMT/toolchains/llvm/prebuilt/darwin-x86_64/bin/armv7a-linux-androideabi30-clang CXX=$NDK_HOMT/toolchains/llvm/prebuilt/darwin-x86_64/bin/armv7a-linux-androideabi30-clang GOOS=linux GOARCH=arm CGO_ENABLED=0 go build
```

## 配置文件`/sdcard/afps_nzlov.conf`

优先级是下面高于上面

```
@import https://gitee.com/nzlov/afps/raw/main/global.conf // 从线上导入配置, 上游更新并不会自动加载
@mode def                                                 // 模式 def 默认，ci 启用自定义延迟(增加耗电),默认模式下依然会读取*的延迟 
package idlefps touchingfps                               // 根据包名配置
package idlefps touchingfps interval                      // 根据包名配置
package/activity idlefps touchingfps                      // 根据activity配置
package/activity idlefps touchingfps interval             // 根据activity配置
* idlefps touchingfps interval                            // 全局
```

* `interval` 不存在默认`1s`,单位为毫秒

### 获取当前`activity`
```
adb shell "dumpsys activity activities | grep mResumedActivity"
```

## 注意
* 配置自动生效，不需要重启
* 触摸后更新为`touchingfps`,停止触摸`interval`后更新为`idlefps`
* GSI类取消强制FPS，官方系统设置为最低刷新率
* 与其他设置刷新率冲突
