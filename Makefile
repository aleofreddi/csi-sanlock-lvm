.PHONY: %

all: all@driverd all@lvmctrld ;

%: %@driverd %@lvmctrld ;

%@driverd: %@lvmctrld
	@$(MAKE) -C driverd $*
%@lvmctrld:
	@$(MAKE) -C lvmctrld $*

test: test@driverd test@lvmctrld
	cat driverd/coverage.txt lvmctrld/coverage.txt > coverage.txt
