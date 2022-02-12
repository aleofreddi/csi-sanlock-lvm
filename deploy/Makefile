.PHONY: %

include ../common.mk

all: build ;

%: %.1.17 %.1.18 %.1.19 %.1.20 %.1.21 %.1.22 %.1.23 ;

build.%:
	$(MAKE) -C kubernetes-$* build

clean.%:
	$(MAKE) -C kubernetes-$* clean

build-image: ;

push-image: ;
