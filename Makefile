LDFLAGS =
LDFLAGS_f2=-ldflags '-w -s $(LDFLAGS)'

all: build
build:
	CGO_ENABLED=0 go build -trimpath $(LDFLAGS_f2) -o onedrive-auth_darwin
linux:
	GOOS=linux CGO_ENABLED=0 go build -trimpath $(LDFLAGS_f2) -o onedrive-auth-linux
	# docker run --rm -v $(PWD):/working reeganexe/upx /working/onedrive-auth-linux
win:
	GOOS=windows CGO_ENABLED=0 go build -trimpath $(LDFLAGS_f2) -o onedrive-auth.exe
	# docker run --rm -v $(PWD):/working reeganexe/upx /working/onedrive-auth.exe
pi:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=5 go build -a -installsuffix cgo -trimpath $(LDFLAGS_f2) -o onedrive-auth-pi
