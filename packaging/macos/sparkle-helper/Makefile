.PHONY: build clean

SPARKLE_FRAMEWORK ?= ../Sparkle.framework

build: sparkle-helper

sparkle-helper: main.m
	clang -fobjc-arc -o sparkle-helper main.m \
		-framework Cocoa \
		-F$(dir $(SPARKLE_FRAMEWORK)) \
		-framework Sparkle \
		-rpath @executable_path/../Frameworks

clean:
	rm -f sparkle-helper
