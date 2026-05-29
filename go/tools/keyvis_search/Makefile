CXX ?= c++
CXXFLAGS ?= -O3 -std=c++23 -Wall -Wextra -Wpedantic
CPPFLAGS ?= -Iinclude
LDFLAGS ?=
LDLIBS ?=

TARGET := keyvis-search
SRC := $(wildcard src/*.cpp)
OBJ := $(SRC:src/%.cpp=build/%.o)
DEP := $(OBJ:.o=.d)

.PHONY: all clean run smoke

all: $(TARGET)

$(TARGET): $(OBJ)
	$(CXX) $(LDFLAGS) $^ $(LDLIBS) -o $@

build/%.o: src/%.cpp
	@mkdir -p $(@D)
	$(CXX) $(CPPFLAGS) $(CXXFLAGS) -MMD -MP -c $< -o $@

run: $(TARGET)
	./$(TARGET)

smoke: $(TARGET)
	./$(TARGET) --version
	./$(TARGET) --help >/dev/null

clean:
	rm -rf build $(TARGET)

-include $(DEP)
