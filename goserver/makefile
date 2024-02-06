OUTPUT_DIR := ./bin

GOAPPSERVER := ./server/appServer/appService.go
NAPPSERVER := appService

allserver := appserver

all: ${allserver}
	@echo "build finish"

#编译规则
appserver: $(GOAPPSERVER)
	go build -gcflags=all="-N -l" -o $(OUTPUT_DIR)/$(NAPPSERVER) $(GOAPPSERVER)

# 删除输出文件
clean:
	rm -rf $(OUTPUT_DIR)/$(NAPPSERVER)

