from sys import argv

def generate_header():
    print("Version 3.1")
    print("# namespace test")
    print("# first-file")

def generate_object():
    print("+ k S 6 lion-0")
    print("+ n test")
    print("+ d 0T4dkNzFvNj6lQeYYlS3WmhnlNs=")
    print("+ s carnivores")
    print("+ g 1")
    print("+ t 0")
    print("+ b 7")
    print("- L list_empty 4 kA==")
    print("- L list_scalars 8 kwECAw==")
    print("- M map_empty 4 gA==")
    print("- M map_scalars 20 gwGiA2ECogNiA6IDYw==")
    print("- L list_nested 12 k5CRAZMBkJEB")
    print("- M map_nested 28 gwGAAoEBogNhA4IBgQGiA2ICgA==")
    print("- M mixed_nested 36 gwGQAoIBkAKBA6IDYQOSgQGiA2GBAqIDYg==")

def generate_scalar():
    print("+ k S 4 key1")
    print("+ n test")
    print("+ d 7JEZLUt/jONdXXjTS8ply6qqyWA=")
    print("+ s demo")
    print("+ g 1")
    print("+ t 428179113")
    print("+ b 3")
    print("- I foo 123")
    print("- S bar 3 abc")
    print("- Z baz T")

def generate_scalar_other_namespace():
    print("+ k S 4 key1")
    print("+ n other")
    print("+ d 7JEZLUt/jONdXXjTS8ply6qqyWA=")
    print("+ s demo")
    print("+ g 1")
    print("+ t 428179113")
    print("+ b 3")
    print("- I foo 123")
    print("- S bar 3 abc")
    print("- Z baz T")

def generate_geojson():
    print("+ k S 4 key1")
    print("+ n test")
    print("+ d 7JEZLUt/jONdXXjTS8ply6qqyWA=")
    print("+ s demo")
    print("+ g 1")
    print("+ t 428179113")
    print("+ b 1")
    print("- G geo 39 {\"type\": \"Point\", \"coordinates\": [1,1]}")

def generate_invalid_object():
    print("+ k S 4 key1")
    print("+ n test")
    print("+ d 7JEZLUt/jONdXXjTS8ply6qqyWA=")
    print("+ s demo")
    print("+ g 1")
    print("+ t 428179113")
    print("+ b 2")
    print("- M map 20 gaVoZWxsb6V3b3JsZA==")
    print("- L list 20 kqVoZWxsb6V3b3JsZA==")

def generate_udf():
    print("* u L test.lua 25 -- just an empty Lua file")

def generate_index():
    print("* i test test-set int-index N 1 int-bin N")

if __name__ == "__main__":
    repeats = int(argv[1])

    generate_header()
    for i in range(repeats):
        generate_object()
        generate_scalar()
        generate_scalar_other_namespace()
        generate_geojson()
        generate_invalid_object()
        generate_udf()
        generate_index()
