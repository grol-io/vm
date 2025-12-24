#include <inttypes.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#ifndef DEBUG
#define DEBUG 0
#endif

#if DEBUG
#define DEBUG_PRINT(fmt, ...)                                                  \
  do {                                                                         \
    fprintf(stderr, fmt, __VA_ARGS__);                                         \
  } while (0)
#else
#define DEBUG_PRINT(fmt, ...)                                                  \
  do {                                                                         \
  } while (0)
#endif

typedef struct {
  int64_t raw_data; // int64_t
} Operation;

uint8_t get_opcode(Operation op) { return (uint8_t)(op.raw_data & 0xFF); }

int64_t get_operand(Operation op) { return (int64_t)(op.raw_data >> 8); }

typedef struct CPU {
  int64_t accumulator;
  int64_t pc;
  Operation *program;
  size_t program_size;
} CPU;

void run_program(CPU *cpu) {
  int64_t end = (int64_t)(cpu->program_size);
  while (cpu->pc < end) {
    Operation op = cpu->program[cpu->pc];
    uint8_t opcode = get_opcode(op);
    int64_t operand = get_operand(op);
    switch (opcode) {
    case 0: // EXIT
      printf("Exit at PC %" PRId64 ": %" PRId64 " code: %" PRIX64 "\n", cpu->pc,
             cpu->accumulator, operand);
      // note that switching to int and using return op.data; adds 1s to
      // linux/amd64 times (2.6s->3.5s) [but not on apple silicon]
      exit(operand);
    case 1: // LOAD
      DEBUG_PRINT("LOAD %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator = operand;
      break;
    case 2: // ADD
      DEBUG_PRINT("ADD %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      cpu->accumulator += operand;
      break;
    case 3: // JNE
      DEBUG_PRINT("JNE %" PRId64 " at PC %" PRId64 "\n", operand, cpu->pc);
      if (cpu->accumulator != 0) {
        cpu->pc = operand;
        continue;
      }
      break;
    default:
      fprintf(stderr, "Unknown opcode %d at PC %" PRId64 "\n", opcode, cpu->pc);
      break;
    }
    cpu->pc++;
  }
  printf("Program finished. Accumulator: %" PRId64 "\n", cpu->accumulator);
}

#define HEADER "\x01GROL VM" // matches cpu.HEADER
#define PACKED_SIZE 8

int main(int argc, char **argv) {
  if (argc < 2) {
    fprintf(stderr, "Usage: %s <program.vm>\n", argv[0]);
    return 1;
  }
  const char *filename = argv[1];
  FILE *f = fopen(filename, "rb");
  if (!f) {
    perror("Failed to open file");
    return 1;
  }
  CPU cpu = {0};
  fseek(f, 0, SEEK_END);
  cpu.program_size =
      ftell(f) / PACKED_SIZE - 1; // packed size of Operation in file - header.
  cpu.program = malloc(cpu.program_size * sizeof(Operation));
  if (!cpu.program) {
    perror("Failed to allocate memory for program");
    fclose(f);
    return 1;
  }
  fseek(f, 0, SEEK_SET);
  char header[strlen(HEADER) + 1];
  if (fread(header, strlen(HEADER), 1, f) != 1) {
    perror("Failed to read header");
    return 1;
  }
  if (strncmp(header, HEADER, strlen(HEADER)) != 0) {
    fprintf(stderr, "Invalid header: %s\n", header);
    return 1;
  }
  if (fread(cpu.program, PACKED_SIZE, cpu.program_size, f) !=
      cpu.program_size) {
    perror("Failed to read operation");
    free(cpu.program);
    fclose(f);
    return 1;
  }
  fclose(f);
  printf("Loaded program with %zu operations\n", cpu.program_size);
  run_program(&cpu);
  free(cpu.program);
  return 0;
}
