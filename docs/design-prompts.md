# vandit.dev redesign: the owner's design prompts

Every design instruction Vandit gave during the October 2026 redesign, in order, in Vandit's own words.
Line breaks became punctuation; the words are unchanged. Answers to multiple-choice questions are marked [answer]. Read "Where it ended up" first: some early
prompts were replaced by later ones.

## Where it ended up (live as of commit 6707641 and later)

- One compact column, black and white only. No dither or shader, no signature, no navbar, no
  two-tone grey sections. The two-column grid from prompt 5 was tried, then dropped for one column.
- Typography: Geist and Geist Mono, following the Pierrick Calvez guide.
- Rows collapse. The title is the link, a chevron beside the title opens the details, external
  link icons sit outside the toggle, and every external link or icon has a hover label.
- Company logos and project icons sit in tiles with double borders.
- "hero is good": do not change the hero.
- Sources used: ui-skills.com CLI, pierrickcalvez.com typography guide, interfaces.dev cheat sheet,
  checklist.design, lawsofux.com, the 12 principles of animation (raphaelsalaja.com),
  designsystemchecklist.com. nikhilwho.in for the vibe only, never copied.

## The prompts

1. anything to update on the website acc. to new changes? also can you use ui-skills.com/cli skills
   on the website to make the website look more good
2. [answer] How far should the UI change go? "the page is too long, i want something compact and no
   fluff site"
3. im thinking better layout and better fonts and better everything ui wise
4. use ui-skills.com/cli skills
5. [answer] Intro ID card overlay: "Remove the overlay". Sections to leave the homepage: "GitHub
   heatmap, Building (indie), Sign here (guestbook), Stack & Tooling wall". Layout: "Two-column
   grid" (later replaced by one column). Fonts: "Geist + Geist Mono, tighter".
6. too simple and no pleasing to eyes. should we remove the shader? use ui-skills.com/cli
7. https://nikhilwho.in/ i like this type of vibe but pls dont copy it
8. https://www.pierrickcalvez.com/journal/a-five-minute-guide-to-better-typography see this and
   follow it for typography
9. use this img you will find in downloads (IMG_20240219_175212_869, now
   assets/images/vandit-portrait.jpg)
10. i dont like the signature on the site
11. and colors should we remove the moving shader to keep things simple and to the point?
12. but i dont like the dither and colors. can we explroe some more things
13. when i say colors i meant the bg black,grey sections that
14. get companies i worked at logo: fluidcloud.com, rapidfort.com, aquanode.io. get their favicon
15. https://interfaces.dev/cheat-sheet follow all tips from this
16. also keep the current color not cyan
17. also double borders look good on things, maybe for favicons you can do that
18. https://www.checklist.design/skill also see this checklist
19. and animations?
20. https://www.raphaelsalaja.com/library/12-principles-of-animation,
    https://www.designsystemchecklist.com/, https://lawsofux.com/ see all that applies and apply.
    focus on small things as much as possible since small moments make it premium
21. for projects section can you add a svg icon related to the project?
22. also my photo you cropped too much
23. now update other pages as well
24. also i feel compact things: sub descriptions should be hidden by default, only expanded when
    clicked on expand icon, and the expand icon should also be hidden by default unless area is
    hovered. also i feel black white contrast would work better wdyt
25. icon for github link can stay outside of expand icon
26. also do the same expand thing on each page. keep links icons outside of expand
27. lets remove navbar have all pages linked from the bio section
28. the copy on the site can be improved
29. also why does on VM in opensource has arrow. all should have. also add hover label on icons or
    links that take outside so user knows
30. ok so the expand icon should be right on the side of title so if i wanna read blog i can click
    the title and go in if i wanna expand i can click the expand or is there any better ux?
31. yes. also see what laws of ux we are not following
32. all gaps fixed?
33. hero is good. remove old css
34. add vawe.dev in projects, and remove the progress status
35. vawe should be under building alongside (ThreadCite and Argus)
