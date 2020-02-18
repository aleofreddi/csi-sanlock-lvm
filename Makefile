.PHONY: %

all: all@driverd all@lvmctrld ;

%: %@driverd %@lvmctrld ;

%@driverd: %@lvmctrld
	@$(MAKE) -C driverd $*
%@lvmctrld:
	@$(MAKE) -C lvmctrld $*
