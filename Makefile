.PHONY: %

include common.mk

%:
	$(MAKE) $(MFLAGS) -C lvmctrld proto $*
	$(MAKE) $(MFLAGS) -C driverd $*
	$(MAKE) $(MFLAGS) -C deploy $*
	[ -r driverd/coverage.txt -a -r lvmctrld/coverage.txt ] && cat driverd/coverage.txt lvmctrld/coverage.txt > coverage.txt || $(RM) coverage.txt
