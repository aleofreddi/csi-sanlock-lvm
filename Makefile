.PHONY: %

all: all@driverd all@lvmctrld all@deploy ;

%: %@driverd %@lvmctrld %@deploy ;

%@driverd: %@lvmctrld
	@$(MAKE) -C driverd $*
%@lvmctrld:
	@$(MAKE) -C lvmctrld $*
%@deploy:
	@$(MAKE) -C deploy/kubernetes $*

build-image: build-image@driverd build-image@lvmctrld ;

push-image: push-image@driverd push-image@lvmctrld ;

test: test@driverd test@lvmctrld
	cat driverd/coverage.txt lvmctrld/coverage.txt > coverage.txt

all@driverd: build@lvmctrld
build@driverd: build@lvmctrld
test@driverd: build@lvmctrld
