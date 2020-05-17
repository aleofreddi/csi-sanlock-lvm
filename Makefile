.PHONY: %
.DEFAULT_GOAL := all

%:
	@for i in lvmctrld driverd deploy/kubernetes; do $(MAKE) $(MFLAGS) -C $$i $*; done

test:
	@for i in lvmctrld driverd deploy/kubernetes; do $(MAKE) $(MFLAGS) -C $$i $*; done
	cat driverd/coverage.txt lvmctrld/coverage.txt > coverage.txt
