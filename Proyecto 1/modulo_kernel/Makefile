obj-m += sysinfo_module.o

all:
	make -C /lib/modules/$(shell uname -r)/build M=/home/fernando/Escritorio/Proyecto_SO1/modulo_kernel modules

clean:
	make -C /lib/modules/$(shell uname -r)/build M=/home/fernando/Escritorio/Proyecto_SO1/modulo_kernel clean