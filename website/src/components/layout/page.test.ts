import { describe, expect, it } from "vitest";
import { canonicalPath } from "./page";

describe("canonicalPath", () => {
  it("TestCanonicalPath_an_indexable_page_names_its_own_page_path", () => {
    expect(canonicalPath("/explorer", false, "/explorer")).toBe("/explorer");
    expect(canonicalPath("/explorer", false, "/explorer/")).toBe("/explorer");
  });

  it("TestCanonicalPath_a_page_kept_out_of_search_names_the_address_it_was_opened_at", () => {
    // The explorer page component renders every /explorer/... address with the landing's metadata.
    expect(canonicalPath("/explorer", true, "/explorer/tx/ab12")).toBe("/explorer/tx/ab12");
    expect(canonicalPath("/explorer", true, "/explorer/custom/")).toBe("/explorer/custom");
  });

  it("TestCanonicalPath_the_root_stays_the_root", () => {
    expect(canonicalPath("/", true, "/")).toBe("/");
  });
});
