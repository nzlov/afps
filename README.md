# 自动刷新率
 
## 构建
```
CC=$NDK_HOMT/toolchains/llvm/prebuilt/darwin-x86_64/bin/armv7a-linux-androideabi30-clang CXX=$NDK_HOMT/toolchains/llvm/prebuilt/darwin-x86_64/bin/armv7a-linux-androideabi30-clang GOOS=linux GOARCH=arm CGO_ENABLED=0 go build
```

## 配置文件
```
package idlefps touchingfps
package/activity idlefps touchingfps
* idlefps touchingfps
```
