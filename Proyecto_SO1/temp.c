#include <linux/module.h>
#include <linux/proc_fs.h>
#include <linux/seq_file.h>
#include <linux/sched.h>
#include <linux/mm.h>
#include <linux/uaccess.h>
#include <linux/slab.h>
#include <linux/cgroup.h>
#include <linux/fs.h>
#include <linux/ktime.h>
#include <linux/timekeeping.h>
#include <linux/time64.h>
#include <linux/blkdev.h>

#define FILE_NAME "sysinfo_202200349"
#define MAX_CMDLINE_LENGTH 1000

static char *get_container_id(struct task_struct *task) {
    char *id = kmalloc(64, GFP_KERNEL);
    if (!id)
        return NULL;

    // Obtener el ID del contenedor desde el cgroup
    if (task->cgroups && task->cgroups->dfl_cgrp && task->cgroups->dfl_cgrp->kn) {
        snprintf(id, 64, "%s", task->cgroups->dfl_cgrp->kn->name);
    } else {
        snprintf(id, 64, "unknown");
    }

    return id;
}

static void get_memory_info(struct seq_file *m) {
    struct sysinfo i;
    si_meminfo(&i);

    unsigned long total_ram = i.totalram * i.mem_unit / (1024 * 1024); // Convertir a MB
    unsigned long free_ram = i.freeram * i.mem_unit / (1024 * 1024); // Convertir a MB
    unsigned long used_ram = total_ram - free_ram;

    seq_printf(m, "\t\"Memory\":");
    seq_printf(m, "{\n\t\t\"total_ram\": %lu,\n", total_ram);
    seq_printf(m, "\t\t\"free_ram\": %lu,\n", free_ram);
    seq_printf(m, "\t\t\"used_ram\": %lu\n", used_ram);
    seq_printf(m, "\t},\n");
}

static unsigned long get_cpu_usage(void) {
    u64 user, nice, system, idle, iowait, irq, softirq;
    unsigned long total_time, idle_time, usage;

    // Leer las estadísticas de CPU
    user = kcpustat_cpu(0).cpustat[CPUTIME_USER];
    nice = kcpustat_cpu(0).cpustat[CPUTIME_NICE];
    system = kcpustat_cpu(0).cpustat[CPUTIME_SYSTEM];
    idle = kcpustat_cpu(0).cpustat[CPUTIME_IDLE];
    iowait = kcpustat_cpu(0).cpustat[CPUTIME_IOWAIT];
    irq = kcpustat_cpu(0).cpustat[CPUTIME_IRQ];
    softirq = kcpustat_cpu(0).cpustat[CPUTIME_SOFTIRQ];

    // Calcular el tiempo total de CPU
    total_time = user + nice + system + irq + softirq + iowait;
    idle_time = idle;

    // Calcular el uso de CPU
    if (total_time == 0)
        return 0;

    usage = 100 - ((idle_time * 100) / total_time);
    return usage;
}

static void get_cpu_info(struct seq_file *m) {
    unsigned long cpu_usage = get_cpu_usage();

    seq_printf(m, "\t\"CPU\":");
    seq_printf(m, "{\n\t\t\"usage\": %lu\n", cpu_usage);
    seq_printf(m, "\t},\n");
}

static int is_docker_container(struct task_struct *task) {
    // Verifica si el proceso está en un cgroup de Docker
    if (task && task->cgroups && strstr(task->cgroups->dfl_cgrp->kn->name, "docker") != NULL) {
        return 1;
    }

    return 0;
}

static void get_container_processes_info(struct seq_file *m) {
    struct task_struct *task;
    bool found = false;

    struct sysinfo i;
    si_meminfo(&i);
    unsigned long total_ram = i.totalram * i.mem_unit; // Total de RAM en bytes

    for_each_process(task) {
        if (is_docker_container(task) && task->group_leader == task) { // Solo el proceso principal
            struct mm_struct *mm = task->mm;
            unsigned long rss = 0;
            unsigned long porc_ram = 0;

            if (mm) {
                down_read(&mm->mmap_lock);
                rss = get_mm_rss(mm) * PAGE_SIZE; // Convertir páginas a bytes
                up_read(&mm->mmap_lock);
            }

            unsigned long total_time = task->utime + task->stime;
            unsigned long cpu_usage = (total_time * 100) / jiffies;

            // Obtener información de I/O
            unsigned long read_bytes = task->ioac.read_bytes;
            unsigned long write_bytes = task->ioac.write_bytes;

            // Obtener el ID del contenedor
            char *container_id = get_container_id(task);

            if (found) {
                seq_printf(m, ",\n");
            }

            seq_printf(m, "\t\t{\n");
            seq_printf(m, "\t\t\t\"pid\": %d,\n", task->pid);
            seq_printf(m, "\t\t\t\"name\": \"%s\",\n", task->comm);
            seq_printf(m, "\t\t\t\"container_id\": \"%s\",\n", container_id);
            porc_ram = (rss * 100) / total_ram; // Calcular porcentaje de memoria
            seq_printf(m, "\t\t\t\"memoryUsage\": %lu.%02lu,\n", porc_ram / 100, porc_ram % 100);
            seq_printf(m, "\t\t\t\"cpuUsage\": %lu.%02lu,\n", cpu_usage / 100, cpu_usage % 100);
            seq_printf(m, "\t\t\t\"diskUsage\": {\"read_bytes\": %lu, \"write_bytes\": %lu},\n", read_bytes, write_bytes);
            seq_printf(m, "\t\t\t\"ioInfo\": {\"read_bytes\": %lu, \"write_bytes\": %lu}\n", read_bytes, write_bytes);
            seq_printf(m, "\t\t}");
            found = true;

            kfree(container_id);
        }
    }
    seq_printf(m, "\n");
    if (!found) {
        seq_printf(m, "{ \"error\": \"No container processes found\" }\n");
    }
}

static int sysinfo_proc_show(struct seq_file *m, void *v) {
    seq_printf(m, "{\n");
    get_memory_info(m);
    get_cpu_info(m);
    seq_printf(m, "\t\"Processes\":");
    seq_printf(m, "[\n");
    get_container_processes_info(m);
    seq_printf(m, "\t]\n");
    seq_printf(m, "}\n");
    return 0;
}

static int sysinfo_proc_open(struct inode *inode, struct file *file) {
    return single_open(file, sysinfo_proc_show, NULL);
}

static const struct proc_ops sysinfo_proc_ops = {
    .proc_open = sysinfo_proc_open,
    .proc_read = seq_read,
    .proc_lseek = seq_lseek,
    .proc_release = single_release,
};

static int __init sysinfo_module_init(void) {
    proc_create(FILE_NAME, 0, NULL, &sysinfo_proc_ops);
    return 0;
}

static void __exit sysinfo_module_exit(void) {
    remove_proc_entry(FILE_NAME, NULL);
}

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Fernando Alvarado");
MODULE_DESCRIPTION("Módulo de kernel que captura métricas del sistema");
MODULE_VERSION("1.0");

module_init(sysinfo_module_init);
module_exit(sysinfo_module_exit);