module Main exposing (..)

import Browser
import Element exposing (..)
import Element.Background as Background
import Element.Border as Border
import Element.Events as Events
import Element.Font as Font
import Element.Input as Input
import Html
import Http
import Json.Decode as D
import Json.Encode as E
import Task
import Time exposing (..)



-- MAIN


main : Program Flags Model Msg
main =
    Browser.element
        { init = init
        , update = update
        , subscriptions = subscriptions
        , view = view
        }



-- MODEL


type ServerState
    = LoadFailed
    | Loading
    | Loaded String


type alias Model =
    { state : ServerState
    , editLimit : Maybe String
    , editStart : Maybe String
    , url : String
    }


type alias Flags =
    { url : String }


type alias Namespace =
    { name : String
    , hasDownQuota : Bool
    , canExtend : Bool
    , memUsed : Int
    , memLimit : Int
    , autoStartHour : Maybe Int
    , remaining : Maybe String
    }


type alias Status =
    { clock : String
    , namespaces : List Namespace
    }


loadState : String -> Cmd Msg
loadState url =
    Http.get
        { url = url ++ "status"
        , expect = Http.expectString GotUpdate
        }


encode : String -> E.Value
encode namespace =
    E.object [ ( "namespace", E.string namespace ) ]


extend : String -> String -> Cmd Msg
extend url namespace =
    Http.post
        { url = url ++ "extend"
        , body = encode namespace |> Http.jsonBody
        , expect = Http.expectString GotUpdate
        }


encodeLimit : String -> Int -> E.Value
encodeLimit namespace limit =
    E.object [ ( "namespace", E.string namespace ), ( "limit", E.int limit ) ]


setLimit : String -> String -> Int -> Cmd Msg
setLimit url namespace value =
    Http.post
        { url = url ++ "setMemLimit"
        , body = encodeLimit namespace value |> Http.jsonBody
        , expect = Http.expectString GotUpdate
        }


encodeStart : String -> Maybe Int -> E.Value
encodeStart namespace value =
    let
        start =
            case value of
                Just x ->
                    E.int x

                Nothing ->
                    E.null
    in
    E.object [ ( "namespace", E.string namespace ), ( "startHour", start ) ]


setStart : String -> String -> Maybe Int -> Cmd Msg
setStart url namespace value =
    Http.post
        { url = url ++ "setStartHour"
        , body = encodeStart namespace value |> Http.jsonBody
        , expect = Http.expectString GotUpdate
        }


init : Flags -> ( Model, Cmd Msg )
init flags =
    ( { state = Loading
      , editLimit = Nothing
      , editStart = Nothing
      , url = flags.url
      }
    , loadState flags.url
    )



-- UPDATE


type Msg
    = GotUpdate (Result Http.Error String)
    | GetUpdate Time.Posix
    | Extend String
    | EditLimit (Maybe String)
    | SetLimit String Int
    | EditStart (Maybe String)
    | SetStart String (Maybe Int)


update : Msg -> Model -> ( Model, Cmd Msg )
update msg model =
    case msg of
        GotUpdate result ->
            case result of
                Ok json ->
                    ( { model | state = Loaded json }, Cmd.none )

                Err _ ->
                    ( { model | state = LoadFailed }, Cmd.none )

        GetUpdate newTime ->
            ( model, loadState model.url )

        Extend namespace ->
            ( model, extend model.url namespace )

        EditLimit namespace ->
            ( { model | editLimit = namespace }, Cmd.none )

        SetLimit namespace limit ->
            ( model, setLimit model.url namespace limit )

        EditStart namespace ->
            ( { model | editStart = namespace }, Cmd.none )

        SetStart namespace start ->
            ( model, setStart model.url namespace start )



-- SUBSCRIPTIONS


subscriptions : Model -> Sub Msg
subscriptions model =
    Time.every 5000 GetUpdate


nsRow : Namespace -> Element msg
nsRow ns =
    row
        [ spacing 20
        , padding 20
        ]
        [ text ns.name
        , text "-"
        , text "State"
        , text (String.fromInt ns.memLimit)
        , text (String.fromInt ns.memUsed)
        , text "-"
        ]


blue =
    rgb255 100 100 255


green =
    rgb255 75 255 75


red =
    rgb255 255 75 75


dark =
    rgb255 20 20 20


grey =
    rgb255 130 130 130


showNamespace : Namespace -> Element Msg
showNamespace ns =
    el [ getColor ns, Font.alignLeft ] <| text ns.name


headerAttr =
    [ Font.size 20, Font.color blue ]


nsTable : Status -> Model -> Element Msg
nsTable status model =
    table
        [ spacing 10
        , padding 10
        , width (fill |> maximum 950)
        , Font.alignRight
        , Font.size 18
        ]
        { data = status.namespaces
        , columns =
            [ { header = el (Font.alignLeft :: headerAttr) <| text "Namespace"
              , width = fillPortion 3
              , view = showNamespace
              }
            , { header = el headerAttr <| text "Memory (Gi)"
              , width = fillPortion 1
              , view = \ns -> el [ getColor ns ] <| text <| String.fromInt ns.memUsed
              }
            , { header = el headerAttr <| text "Limit (Gi)"
              , width = fillPortion 1
              , view = showLimit model
              }
            , { header = el headerAttr <| text "Auto Start"
              , width = fillPortion 1
              , view = showStart model
              }
            , { header = el headerAttr <| text "Remaining"
              , width = fillPortion 1
              , view = showRemaining
              }
            , { header = none
              , width = shrink
              , view = extendButton
              }
            , { header = none
              , width = fillPortion 1
              , view = \ns -> none
              }
            ]
        }


startString : Maybe Int -> String
startString value =
    case value of
        Nothing ->
            "-"

        Just x ->
            String.fromInt x


showStart : Model -> Namespace -> Element Msg
showStart model ns =
    el
        [ Font.alignRight
        , Events.onMouseEnter <| EditStart <| Just ns.name
        , Events.onMouseLeave <| EditStart Nothing
        , getColor ns
        ]
    <|
        if editing model.editStart ns.name then
            startEditor ns.name ns.autoStartHour

        else
            text <| startString ns.autoStartHour


decAutostart : Maybe Int -> Maybe Int
decAutostart value =
    case value of
        Nothing ->
            Just 23

        Just x ->
            if x == 0 then
                Nothing

            else
                Just <| x - 1


incAutostart : Maybe Int -> Maybe Int
incAutostart value =
    case value of
        Nothing ->
            Just 0

        Just x ->
            if x == 23 then
                Nothing

            else
                Just <| x + 1


startEditor : String -> Maybe Int -> Element Msg
startEditor namespace value =
    let
        decButton =
            Input.button []
                { onPress = Just <| SetStart namespace <| decAutostart value
                , label = text "< "
                }

        incButton =
            Input.button []
                { onPress = Just <| SetStart namespace <| incAutostart value
                , label = text " >"
                }
    in
    row [ alignRight ]
        [ decButton
        , text (startString value)
        , incButton
        ]


editing : Maybe String -> String -> Bool
editing flag name =
    case flag of
        Just ns ->
            ns == name

        _ ->
            False


limitEditor : String -> Int -> Element Msg
limitEditor namespace value =
    let
        decButton =
            if value > 10 then
                Input.button []
                    { onPress = Just <| SetLimit namespace <| value - 10
                    , label = text "< "
                    }

            else
                el [ Font.color grey ] <| text "< "

        incButton =
            if value < 100 then
                Input.button []
                    { onPress = Just <| SetLimit namespace <| value + 10
                    , label = text " >"
                    }

            else
                el [ Font.color grey ] <| text " >"
    in
    row [ alignRight ]
        [ decButton
        , text (String.fromInt value)
        , incButton
        ]


showLimit : Model -> Namespace -> Element Msg
showLimit model ns =
    el
        [ getColor ns
        , Font.alignRight
        , Events.onMouseEnter <| EditLimit <| Just ns.name
        , Events.onMouseLeave <| EditLimit Nothing
        ]
    <|
        if editing model.editLimit ns.name then
            limitEditor ns.name ns.memLimit

        else
            text <| String.fromInt ns.memLimit


padAndMod : Int -> String
padAndMod val =
    String.fromInt (modBy 60 val) |> String.padLeft 2 '0'


formatRemaining : Int -> String
formatRemaining millis =
    let
        m =
            millis // 1000 // 60

        h =
            m // 60

        hs =
            if h > 0 then
                String.fromInt h ++ "h "

            else
                ""

        ms =
            if m > 0 && h > 0 then
                padAndMod m ++ "m"

            else if m > 0 then
                String.fromInt m ++ "m"

            else
                ""
    in
    hs ++ ms


getColor : Namespace -> Attribute msg
getColor ns =
    if ns.hasDownQuota && ns.memUsed > 0 then
        Font.color red

    else if ns.memUsed > ns.memLimit then
        Font.color red

    else if not ns.hasDownQuota then
        Font.color green

    else
        Font.color grey


showRemaining : Namespace -> Element Msg
showRemaining ns =
    case ns.remaining of
        Nothing ->
            none

        Just x ->
            el [ Font.alignRight, getColor ns ] <| text x


extendButton : Namespace -> Element Msg
extendButton ns =
    if ns.canExtend then
        Input.button
            [ alignRight
            , Border.width 0
            , Border.rounded 3
            ]
            { onPress = Just <| Extend ns.name, label = text ">" }

    else
        none


errorPage : String -> Element Msg
errorPage message =
    el [] (text message)


title : String -> Element Msg
title clock =
    row [ width fill ]
        [ el [ Font.size 40 ] <| text "Pod Reaper"
        , el [ Font.size 14, Font.alignRight, width (fill |> maximum 610) ] <|
            text clock
        ]


page : Status -> Model -> Element Msg
page status model =
    column
        [ Font.color grey
        , spacing 20
        , padding 20
        , width fill
        , height fill
        ]
        [ title status.clock
        , nsTable status model
        ]


defaultStyle =
    [ Background.color dark
    , Font.color grey
    ]


nsDecoder : D.Decoder Namespace
nsDecoder =
    D.map7 Namespace
        (D.field "name" D.string)
        (D.field "hasDownQuota" D.bool)
        (D.field "canExtend" D.bool)
        (D.field "memUsed" D.int)
        (D.field "memLimit" D.int)
        (D.maybe <| D.field "autoStartHour" D.int)
        (D.maybe <| D.field "remaining" D.string)


statusDecoder : D.Decoder Status
statusDecoder =
    D.map2 Status
        (D.field "clock" D.string)
        (D.field "namespaces" (D.list nsDecoder))


msgDecoder : String -> Result D.Error Status
msgDecoder json =
    D.decodeString statusDecoder json


view : Model -> Html.Html Msg
view model =
    case model.state of
        LoadFailed ->
            layout defaultStyle <| errorPage "Unable to retrieve namespace data"

        Loading ->
            layout defaultStyle <| errorPage "Loading..."

        Loaded json ->
            let
                status =
                    case msgDecoder json of
                        Ok values ->
                            values

                        Err x ->
                            { namespaces = [], clock = "JSON decoding error: " ++ D.errorToString x }
            in
            layout defaultStyle <| page status model
