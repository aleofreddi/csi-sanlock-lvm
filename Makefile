.PHONY: %

include common.mk

%:
	$(MAKE) $(MFLAGS) -C lvmctrld proto $*
	$(MAKE) $(MFLAGS) -C driverd $*
	$(MAKE) $(MFLAGS) -C deploy $*

test:
	$(MAKE) $(MFLAGS) -C lvmctrld $*
	$(MAKE) $(MFLAGS) -C driverd $*
	$(MAKE) $(MFLAGS) -C deploy $*
	cat driverd/coverage.txt lvmctrld/coverage.txt > coverage.txt
