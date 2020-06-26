#----------------------------
APP=PagerdutyNotifier
APPDIR=dist/$(APP).app
EXECUTABLE=$(APPDIR)/Contents/MacOS/notifier
ICONFILE=$(APPDIR)/Contents/Resources/PagerDuty.icns

build: $(EXECUTABLE) $(ICONFILE)

$(EXECUTABLE): src/*.go
	go build -o "$@" $^

run:
	open $(APPDIR)

install:
	cp -r $(APPDIR) /Applications

icon-clear-cache:
	sudo rm -rfv /Library/Caches/com.apple.iconservices.store
	sudo find /private/var/folders/ \( -name com.apple.dock.iconcache -or -name com.apple.iconservices \) -exec rm -rfv {} \;
	sleep 3
	sudo touch /Applications/*
	killall Dock; killall Finder

$(ICONFILE): assets/pd-logo.png
	rm -rf assets/pd.iconset
	mkdir -p assets/pd.iconset
	for size in 16 32 64 128 256 512 1024; do \
	   sips -z $$size $$size assets/pd-logo.png --out assets/pd.iconset/icon_$${size}x$${size}.png; \
	done
	iconutil -c icns -o $(ICONFILE) assets/pd.iconset

clean:
	rm -rf package
	rm -rf assets/pd.iconset
	rm -f assets/pd.icns
	rm -f $(EXECUTABLE)
	rm -f $(ICONFILE)
	rm -f dist/Applications

dmg: build
	ln -fs /Applications dist
	hdiutil create -volname $(APP) -srcfolder ./dist -ov ${PACKAGE}

# Some pre-requisits for building this project
install-dependencies:
	go mod download

